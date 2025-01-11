### Purpose

This tiny library is intended to prevent large slices from busting the memory.
It implements a 2 part slice:
- the first part - the head, starting with the index value 0 lives in memory
- the second part - the tail - lives on the disk

The Slicer interface usage is as follows:

```bash
type Slicer[T any] interface {
	// Appends: appends the elements to the Slicer as to a regular slice
	Append(element ...T) error
	// Len: returns the number of elements
	Len() int
	// Get: retrieves an element at the index. Similar to slice[i]
	Get(index int) (T, error)
	// Put: stores a value at the index. Similar to slice[index]=element
	Put(index int, element T) error
	// Slice: returns a subslice. Maximum 2 parameters: start and end
	// similar to slice[start:end]. If only one parameter is given,
	// it is interpreted as start. So, Slice(3) ~ slice[3:]
	Slice(ind ...int) ([]T, error)
	// Delete: deletes the "count" of elements starting with slice[start]
	Delete(start, count int) error
	// Cleanup: stops the go routine that is tasked with disk cleanup
	// necessitated by the Delete calls.
	Cleanup()
	// other methods
}
```

You create a Slicer by calling

```bash
// New created a Slicer object. It accepts 2 parameters:
// slice:  an actual slice, that will live in memory. So, the Slicer
// memory footprint will be cap(slice).
// rootPath:  the path on the disk where the Slicer tail will live.
// a randomly named subdir will be created, so multiple Slicers
// with the same rootPath (e.g. system temp directory) won't collide
func New[T any](slice []T, rootPath string) (Slicer[T], error)
```

When you are done with a slicer, call slicer.Cleanup() to clean the disk and stop a go routine