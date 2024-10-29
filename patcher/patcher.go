// this package focuses on patching the files
// apply a patch to a document
// jsonprocessor will process any operations on two JSON objects.
// it can compare arbitrary JSON objects.
// encoded JSON objects should be unmarshalled into a variable of type any.
package patcher

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
)

// Patch is a struct that represents a patch to be applied to a document
type Patch struct {
	Operation string      // the operation to be performed
	Path      string      // the path to the value to be patched
	Value     interface{} // the value to be added or replaced
}

// A recursive struct that represents a patch to be applied to a document
type patchVisitor struct {
	patch       Patch  // the patch to be applied
	currentPath string // the remaining path to the value to be patched
}

// A PatchResponse stores the response from a Patch operation
type PatchResponse struct {
	Uri         string `json:"uri"`         // The URI at which this patch was applied.
	PatchFailed bool   `json:"patchFailed"` // A boolean indicating whether this patch failed.
	Message     string `json:"message"`     // A message indicating why a patch failed or "patches applied."
}

// A JSONProcessor is used by ProcessJSON to handle arbitrary values that only contain
// data of valid JSON types. The Map and Slice methods may recursively call
// ProcessJSON on their constituent elements. They may also use DeepEqualJSON to compare
// their constituent elements with other JSON values.
type JSONProcessor[T any] interface {
	Map(map[string]any) (T, error)
	Slice([]any) (T, error)
	Bool(bool) (T, error)
	Number(float64) (T, error)
	String(string) (T, error)
	Null() (T, error)
}

// create a new patch visitor
func new(patch Patch) (patchVisitor, error) {
	visitor := patchVisitor{}
	visitor.patch = patch
	currentPath, found := strings.CutPrefix(patch.Path, "/")

	if !found {
		slog.Info("Patch missing leading slash", "path", patch.Path)
		return visitor, errors.New("missing leading slash")
	}

	visitor.currentPath = currentPath
	return visitor, nil
}

// apply the patch to the document
func ApplyPatch(doc interface{}, patch Patch) (interface{}, error) {
	slog.Info("patcher ApplyPatch: Applying patch", "patch", patch)
	patcher, err := new(patch)
	if err != nil {
		return nil, err
	}
	slog.Info("patcher ApplyPatch: type of doc", "type", reflect.TypeOf(doc))
	// slog.Info("patcher ApplyPatch: type of patcher", "type", reflect.TypeOf(patcher))

	patchedDoc, err := Accept(doc, &patcher)
	return patchedDoc, err
}

// handle visiting a JSON object with the patch struct
func (p *patchVisitor) Map(m map[string]any) (any, error) {
	slog.Info("patcher Map: Visiting map", "map", m, "patch", p.patch)
	result := make(map[string]any)

	slog.Info("patcher Map: current path", "path", p.currentPath)

	// process the string
	splittedPath := strings.SplitAfterN(p.currentPath, "/", 2)
	slog.Info("patcher Map: splitted path", "path", splittedPath)

	// top level key
	targetKey := strings.TrimSuffix(splittedPath[0], "/")

	// store the rest of the path in the patch visitor
	// if empty, we are at the end at the target location
	if len(splittedPath) == 1 && p.patch.Operation == "ObjectAdd" {
		// check if the key already exists
		for key, val := range m {
			if key == targetKey {
				slog.Info("patcher Map: Key already exists in map", "key", targetKey)
				return m, nil
			}

			result[key] = val
		}

		slog.Info("patcher Map: Key not found in map", "key", targetKey)

		// if not, add the key
		result[targetKey] = p.patch.Value
		slog.Debug("Added key to map", "key", targetKey, "value", p.patch.Value)
		return result, nil
	} else if len(splittedPath) != 1 {
		// update the current path
		p.currentPath = splittedPath[1]
	} else {
		p.currentPath = ""
	}

	found := false
	for key, val := range m {
		if key == targetKey {
			updated, err := Accept(val, p)

			if err != nil {
				return updated, err
			}

			slog.Debug("Updated value in map", "key", key, "value", updated)
			result[key] = updated
			found = true
		} else {
			result[key] = val
		}
	}

	slog.Info("patcher Map: Map after patch", "map", result)

	if !found {
		// if the key is not found, return an error
		slog.Info("patcher Map: Key not found in map", "key", targetKey)
		message := fmt.Sprintf("Key %s not found in map", targetKey)
		return m, errors.New(message)
	} else {
		return result, nil
	}
}

// handle visiting a slice with the patch struct
func (p *patchVisitor) Slice(slice []any) (any, error) {
	slog.Debug("Visiting slice", "slice", slice, "patch", p.patch)
	if p.patch.Operation == "ArrayAdd" && p.currentPath == "" {
		array := append(slice, p.patch.Value)
		slog.Info("Added value to slice", "value", p.patch.Value)
		return array, nil
	} else if p.patch.Operation == "ArrayRemove" && p.currentPath == "" {
		// handle removing an element from the array
		for i, val := range slice {
			if Equal(val, p.patch.Value) {
				array := append(slice[:i], slice[i+1:]...)
				slog.Info("Removed value from slice", "value", p.patch.Value)
				return array, nil
			}
		}
		return slice, nil
	} else if p.currentPath == "" {
		return nil, errors.New("invalid patch operation")
	} else {
		result := make([]any, 0)

		// process the string
		splittedPath := strings.SplitAfterN(p.currentPath, "/", 2)

		// top level key
		targetKey := strings.TrimSuffix(splittedPath[0], "/")

		// convert the index to an integer
		targetIndex, err := strconv.Atoi(targetKey)

		if err != nil {
			return slice, errors.New("invalid index")
		}

		if targetIndex >= len(slice) {
			return slice, errors.New("index out of bounds")
		}

		if len(splittedPath) == 1 {
			return slice, errors.New("path ends in slice")
		}

		// update the current path
		p.currentPath = splittedPath[1]

		// iterate over the slice
		for i, val := range slice {
			if i == targetIndex {
				updated, err := Accept(val, p)

				if err != nil {
					// return the error if there is one
					return updated, err
				}

				slog.Debug("Updated value in slice", "index", i, "value", updated)
				result = append(result, updated)
			} else {
				result = append(result, val)
			}
		}

		return result, nil
	}
}

// handle visiting a boolean with the patch struct
func (p *patchVisitor) Bool(b bool) (any, error) {
	slog.Debug("Visiting boolean", "bool", b, "patch", p.patch)
	return nil, errors.New("patching a boolean is not supported")
}

// handle visiting a number with the patch struct
func (p *patchVisitor) Number(n float64) (any, error) {
	slog.Debug("Visiting number", "number", n, "patch", p.patch)
	return nil, errors.New("patching a number is not supported")
}

// handle visiting a string with the patch struct
func (p *patchVisitor) String(s string) (any, error) {
	slog.Debug("Visiting string", "string", s, "patch", p.patch)
	return nil, errors.New("patching a string is not supported")
}

// handle visiting a null with the patch struct
func (p *patchVisitor) Null() (any, error) {
	slog.Debug("Visiting null", "patch", p.patch)
	return nil, errors.New("patching a null is not supported")
}

// Equal returns true if the inputs val1 and val2 are deeply equal and false
// otherwise. This is meant to be used on unmarshaled JSON values.
func Equal(value1, value2 any) bool {
	return reflect.DeepEqual(value1, value2)
}

// Accept applies the given input visitor to the given input value by calling
// the appropriate visitor method given the type of the input value. Returns an
// error if value is of a type that is not a valid JSON type or if the visitor
// method returns an error.
func Accept[T any](value any, processer JSONProcessor[T]) (T, error) {
	slog.Info("patcher Accept: Accepting visitor", "value", value)
	switch val := value.(type) {
	case map[string]any:
		slog.Info("patcher Accept: Visiting map", "map", val)
		return processer.Map(val)
	case []any:
		return processer.Slice(val)
	case float64:
		return processer.Number(val)
	case bool:
		return processer.Bool(val)
	case string:
		return processer.String(val)
	case nil:
		return processer.Null()
	default:
		var zero T
		return zero, fmt.Errorf("invalid JSON value: %v", value)
	}
}
