// this package is an implementation of a skip list data structure.
package skiplist

import (
	"cmp"
	"context"
	"log/slog"
	"math/rand/v2"
	"sync"
	"sync/atomic"
)

// the node struct
type node[K cmp.Ordered, V any] struct {
	sync.Mutex                               // lock for the node
	key         K                            // key
	value       V                            // value
	next        []atomic.Pointer[node[K, V]] // next pointers
	marked      atomic.Bool                  // mark bit for deletion
	fullyLinked atomic.Bool                  // fully linked bit for insertion
	topLevel    int                          // top level of the node
}

// the skip list struct
type SkipList[K cmp.Ordered, V any] struct {
	head            *node[K, V]   // head node
	totalOperations *atomic.Int32 // total operations
}

// A struct encapsulating the key and value returned by the query
type Pair[K cmp.Ordered, V any] struct {
	Key   K
	Value V
}

// A function that determines whether to update a value given a key's current value
type UpdateCheck[K cmp.Ordered, V any] func(key K, currValue V, exists bool) (newValue V, err error)

// the default max / min values for strings, and default level of skip list
const (
	STRINGMAX     = string(rune(256))
	STRINGMIN     = ""
	DEFAULT_LEVEL = 5
)

// create an empty skip list
func New[K cmp.Ordered, V any](min, max K, maxLevel int) SkipList[K, V] {
	var head, tail node[K, V]

	// Construct head node
	head.key = min
	head.next = make([]atomic.Pointer[node[K, V]], maxLevel)
	head.marked = atomic.Bool{}
	head.fullyLinked = atomic.Bool{}
	head.marked.Store(false)
	head.topLevel = maxLevel - 1 // index starts from 0

	// Construct tail node
	tail.key = max
	tail.next = make([]atomic.Pointer[node[K, V]], maxLevel)
	tail.marked = atomic.Bool{}
	tail.fullyLinked = atomic.Bool{}
	tail.marked.Store(false)
	tail.fullyLinked.Store(true)
	tail.topLevel = 0 // index starts from 0

	// Link head to tail
	for i := 0; i < maxLevel; i++ {
		head.next[i].Store(&tail)
		tail.next[i].Store(nil)
	}
	// set the head to be fully linked
	head.fullyLinked.Store(true)

	// Construct the skip list
	var result SkipList[K, V]
	result.head = &head
	result.totalOperations = new(atomic.Int32)
	result.totalOperations.Store(0)

	return result
}

// create a new node
func newNode[K cmp.Ordered, V any](key K, value V, topLevel int) *node[K, V] {
	var newNode node[K, V]

	newNode.key = key
	newNode.value = value
	newNode.next = make([]atomic.Pointer[node[K, V]], topLevel+1)
	newNode.marked = atomic.Bool{}
	newNode.fullyLinked = atomic.Bool{}
	newNode.marked.Store(false)
	newNode.fullyLinked.Store(false)
	newNode.topLevel = topLevel

	return &newNode
}

// find the value corresponding to the key
func (s *SkipList[K, V]) Find(key K) (V, bool) {
	slog.Debug("Find: searching for key", "key", key) // log the search

	levelFound, _, successor := s.find(key) // find the node

	if levelFound == -1 {
		var nothing V
		return nothing, false
	}

	found := successor[levelFound]
	return found.value, found.fullyLinked.Load() && !found.marked.Load()
}

// update or insert a key value pair in the skip list
func (s *SkipList[K, V]) Upsert(key K, check UpdateCheck[K, V]) (updated bool, err error) {
	slog.Debug("Upsert: upserting key", "key", key) // log the upsert

	// pick a random level for the new node
	topLevel := 0
	for rand.Float32() < 0.5 && topLevel < s.head.topLevel {
		topLevel += 1
	}

	slog.Debug("Upsert: selected top level", "level", topLevel, "key", key) // log the level

	// keep trying until the key is inserted or updated
	for {
		// check if the key exists
		levelFound, preds, succs := s.find(key)
		slog.Debug("Upsert: found level", "level", levelFound) // log the level
		slog.Info("Upsert: found level", "level", levelFound)  // log the level
		if levelFound != -1 {
			found := succs[levelFound]
			if !found.marked.Load() {
				// the key exists (update)
				// need to wait for the node to be fully linked
				slog.Info("Upsert: waiting for node to be fully linked") // log the wait
				for !found.fullyLinked.Load() {
				}

				// lock the node
				found.Lock()

				// use the update check function to update the value
				newValue, err := check(found.key, found.value, true)
				if err != nil {
					found.Unlock()
					return false, err
				} else {
					found.value = newValue
					found.Unlock()
					s.totalOperations.Add(1)
					return true, nil // return true for update
				}
			}
			// the key is marked for deletion, retry
			continue
		}

		// the key does not exist (insert)
		// create a new node
		// zero value for the new node
		slog.Info("Upsert: key not found, creating new node") // log the create
		var def V
		newValue, err := check(key, def, false)
		if err != nil {
			return false, err
		}

		valid := true
		level := 0
		prevKey := key
		used := make([]int, 0)

		// lock the predecessors
		for ; level <= topLevel && valid; level++ {
			// ensure that preds and succs have enough elements
			// if level >= len(preds) || level >= len(succs) {
			// 	valid = false
			// 	break
			// }

			// selectively lock the predecessors
			if preds[level].key < prevKey {
				preds[level].Lock()
				prevKey = preds[level].key
				used = append(used, level)
			}

			// check pred/succ still valid
			unmarked := !preds[level].marked.Load() && !succs[level].marked.Load()
			connected := preds[level].next[level].Load() == succs[level]
			valid = unmarked && connected
		}

		if !valid {
			// pred/succ not valid, unlock and retry
			for _, i := range used {
				preds[i].Unlock()
			}
			continue
		}

		// create the new node
		newNode := newNode(key, newValue, topLevel)
		slog.Debug("Upsert: preds and succs lengths", "preds", len(preds), "succs", len(succs)) // log the lengths
		slog.Debug("newNode next length", "next", len(newNode.next))                            // log the next length

		// link the new node
		for level = 0; level <= topLevel; level++ {
			slog.Info("skiplist Upsert: linking new node", "level", level) // log the link
			newNode.next[level].Store(succs[level])
		}

		// Add to the skip list from the bottom up
		for level = 0; level <= topLevel; level++ {
			slog.Debug("Upsert: adding new node", "level", level) // log the add
			preds[level].next[level].Store(newNode)
		}

		// set the new node to be fully linked
		newNode.fullyLinked.Store(true)

		// unlock the predecessors
		for _, i := range used {
			preds[i].Unlock()
		}

		s.totalOperations.Add(1)
		slog.Info("skiplist Upsert: new node added successfully") // log the success

		return false, nil // return false for insert
	}
}

// remove a key value pair from the skip list
func (s *SkipList[K, V]) Remove(key K) (V, bool) {
	slog.Debug("Delete: deleting key", "key", key) // log the delete

	isMarked := false
	topLevel := -1
	var victim *node[K, V]
	var nothing V

	// keep trying until the key is removed
	for {
		// find the victim node
		levelFound, preds, succs := s.find(key)

		if levelFound != -1 {
			victim = succs[levelFound]
		}

		if !isMarked {
			// first time through
			if levelFound == -1 {
				slog.Info("skiplist Remove: key not found")
				// no matching node found
				return nothing, false
			}

			if !victim.fullyLinked.Load() {
				slog.Info("skiplist Remove: victim not fully linked")
				// victim not fully linked, retry
				return nothing, false
			}

			if victim.marked.Load() {
				slog.Info("skiplist Remove: victim already removed")
				// node already removed
				return nothing, false
			}

			if victim.topLevel != levelFound {
				// victim not fully linked when found
				return nothing, false
			}

			topLevel = victim.topLevel
			victim.Lock()

			if victim.marked.Load() {
				slog.Info("skiplist Remove: victim already removed by another operation")
				// another remove operation has already removed the node
				victim.Unlock()
				return nothing, false
			}

			victim.marked.Store(true)
			isMarked = true
		}

		// victim found, lock the predecessors
		slog.Info("skiplist Remove: victim found, locking predecessors")
		level := 0
		valid := true
		prevKey := key
		used := make([]int, 0)

		for (level <= topLevel) && valid {
			pred := preds[level]

			// selectively lock the predecessors
			if pred.key < prevKey {
				pred.Lock()
				used = append(used, level)
				prevKey = pred.key
			}

			successor := pred.next[level].Load() == victim
			valid = !pred.marked.Load() && successor
			level += 1
		}

		if !valid {
			// pred/succ not valid, unlock and retry
			for _, i := range used {
				preds[i].Unlock()
			}

			continue
		}

		// all preds and succs are locked and valid, unlink
		level = topLevel
		for level >= 0 {
			preds[level].next[level].Store(victim.next[level].Load())
			level -= 1
		}

		// unlock the predecessors and victim
		victim.Unlock()
		for _, i := range used {
			preds[i].Unlock()
		}

		s.totalOperations.Add(1)
		slog.Info("skiplist Remove: node removed successfully")

		return victim.value, true
	}
}

// helper function to find the node
func (s *SkipList[K, V]) find(key K) (int, []*node[K, V], []*node[K, V]) {
	slog.Info("skiplist find: searching for key", "key", key) // log the search

	// initialize the variables for searching the list
	foundLevel := -1
	pred := s.head
	level := s.head.topLevel

	// initialize preds and succs to have length equalt to max level + 1
	preds := make([]*node[K, V], s.head.topLevel+1)
	succs := make([]*node[K, V], s.head.topLevel+1)

	// start from the top level, and find the successor at each level
	for level >= 0 {
		slog.Info("skiplist find: searching level", "level", level) // log the level
		// current node
		current := pred.next[level].Load()

		// search for the key in the current level
		for current.key < key {
			pred = current
			current = pred.next[level].Load()
		}

		// if we found the key, update the highest level found, which is useful for insert and delete
		if current.key == key {
			slog.Info("skiplist find: key found, update highest level found", "key", key, "level", level) // log the key
			if foundLevel == -1 || foundLevel < level {
				foundLevel = level
			}
		}

		// update the predecessors and successors
		preds[level] = pred
		succs[level] = current

		// move to the next level
		level -= 1
	}

	return foundLevel, preds, succs

}

// Need to implement Query function

// multiple-pass query function; find all keys in the range [start, end]
func (s *SkipList[K, V]) Query(context context.Context, start K, end K) (results []Pair[K, V], err error) {
	slog.Debug("Query: querying for keys in range", "start", start, "end", end) // log the query

	// repeat the query
	for {
		// use a counter to check if the write operation is successful
		oldOpearations := s.totalOperations.Load()
		results := s.query(start, end)
		if oldOpearations == s.totalOperations.Load() {
			return results, nil
		}

		// if deadline is reached, return the results
		select {
		case <-context.Done():
			return nil, context.Err()
		}
	}
}

// helper function to query the skip list; single pass
func (s *SkipList[K, V]) query(start K, end K) []Pair[K, V] {
	// initialize the return value
	var results []Pair[K, V]

	// linear search at the bottom level
	current := s.head.next[0].Load()
	for {
		next := current.next[0].Load()
		if current.key < start {
			current = current.next[0].Load()
		} else if current.key > end || next == nil {
			break
		} else {
			results = append(results, Pair[K, V]{current.key, current.value})
			current = next
		}
	}

	return results
}
