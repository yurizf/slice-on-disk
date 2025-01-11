package slice_on_disk

import (
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
)

const GetError = "could not retrive element: %s"
const CLEANUP = -999999

var IndexOutOfBounds = errors.New("index out of bounds")

// Slicer is an interface to work with an object similar to a slice
// whose head is in memory and potentially long tail is on the disk
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

type config[T any] struct {
	slice     []T
	diskSlice []int
	rootPath  string
	diskIndex int
	ch        chan int
}

// New created a Slicer object. It accepts 2 parameters:
// slice:  an actual slice, that will live in memory. So, the Slicer
// memory footprint will be cap(slice).
// rootPath:  the path on the disk where the Slicer tail will live.
// a randomly named subdir will be created, so multiple Slicers
// with the same rootPath (e.g. system temp directory) won't collide
func New[T any](slice []T, rootPath string) (Slicer[T], error) {
	stat, err := os.Stat(rootPath)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", rootPath)
	}

	// verify permissions
	rnd := rand.Intn(100)
	testFname := filepath.Join(rootPath, fmt.Sprintf("probe-%d", rnd))
	if err = os.WriteFile(testFname, []byte("Hello"), 0755); err != nil {
		return nil, err
	}
	defer os.Remove(testFname)

	rootPath, err = os.MkdirTemp(rootPath, "diskslice")
	if err != nil {
		return nil, err
	}

	c := &config[T]{
		slice:     slice,
		diskSlice: make([]int, 0, 4096),
		rootPath:  rootPath,
		diskIndex: cap(slice),
		ch:        make(chan int, 1024),
	}

	// cleaner
	go func() {
		select {
		case val := <-c.ch:
			if val == CLEANUP {
				os.RemoveAll(rootPath)
				return
			}
			fpath := filepath.Join(c.rootPath, fmt.Sprintf("%d", val))
			err := os.Remove(fpath)
			if err != nil {
				log.Printf("error removing file %s: %s", fpath, err.Error())
			}
		default:
		}
	}()

	return c, nil
}

func (c *config[T]) write(id int, t T) error {
	fname := fmt.Sprintf("%d", id)
	f, err := os.Create(filepath.Join(c.rootPath, fname))
	if err != nil {
		return err
	}
	defer f.Close()
	e := gob.NewEncoder(f)
	err = e.Encode(t)
	return err
}

func (c *config[T]) read(fname string) (T, error) {
	var retVal T

	f, err := os.Open(filepath.Join(c.rootPath, fname))
	if err != nil {
		return retVal, fmt.Errorf(GetError, err.Error())
	}

	defer f.Close()
	decoder := gob.NewDecoder(f)
	err = decoder.Decode(&retVal)
	if err != nil {
		return retVal, fmt.Errorf(GetError, err.Error())
	}
	return retVal, nil
}

func (c *config[T]) Append(elements ...T) error {
	for _, e := range elements {
		if len(c.slice) < cap(c.slice) {
			c.slice = append(c.slice, e)
			return nil
		}

		if err := c.write(c.diskIndex, e); err != nil {
			return err
		}

		c.diskSlice = append(c.diskSlice, c.diskIndex)
		c.diskIndex++
	}
	return nil
}

func (c *config[T]) Len() int {
	if len(c.diskSlice) == 0 {
		return len(c.slice)
	}
	return len(c.slice) + len(c.diskSlice)
}

func (c *config[T]) Get(index int) (T, error) {
	var retVal T
	var err error
	if index < 0 || index >= len(c.diskSlice)+len(c.slice) {
		return retVal, IndexOutOfBounds
	}

	if index < len(c.slice) {
		return c.slice[index], nil
	}

	index = index - len(c.slice)

	retVal, err = c.read(fmt.Sprintf("%d", c.diskSlice[index]))
	if err != nil {
		return retVal, fmt.Errorf(GetError, err.Error())
	}

	return retVal, nil
}

func (c *config[T]) Put(index int, element T) error {
	if index >= len(c.diskSlice)+len(c.slice) || index < 0 {
		return IndexOutOfBounds
	}

	if index < len(c.slice) {
		c.slice[index] = element
		return nil
	}

	index = index - len(c.slice)
	return c.write(c.diskSlice[index], element)
}

func (c *config[T]) Slice(ind ...int) ([]T, error) {
	if len(ind) > 2 {
		return nil, fmt.Errorf("invalid number of parameters: %d", len(ind))
	}
	start := 0
	end := len(c.slice) + len(c.diskSlice)
	if len(ind) == 1 {
		start = ind[0]
	}
	if len(ind) == 2 {
		start = ind[0]
		end = ind[1]
	}

	if start < 0 || start > len(c.slice)+len(c.diskSlice) || end < 0 || end > len(c.slice)+len(c.diskSlice) {
		return nil, IndexOutOfBounds
	}

	// https://go.dev/ref/spec#Appending_and_copying_slices
	// The number of elements copied is the minimum of len(src) and len(dst)
	retval := make([]T, end-start, end-start)
	var n int
	if start < len(c.slice) {
		if end >= len(c.slice) {
			n = copy(retval, c.slice[start:])
		} else {
			n = copy(retval, c.slice[start:end])
		}
		start = len(c.slice)
	}

	for i := start; i < end; i++ {
		t, err := c.Get(i)
		if err != nil {
			return nil, err
		}
		retval[n] = t
		n++
	}
	return retval, nil
}

func (c *config[T]) Delete(start, n int) error {
	if start < 0 || start+n > c.Len() {
		return fmt.Errorf("invalid parameters start=%d, todelete=%d for the slice of length %d", start, n, c.Len())
	}

	if start < len(c.slice) {
		if start+n <= len(c.slice) {
			copy(c.slice[start:], c.slice[start+n:])
			c.slice = c.slice[:len(c.slice)-n]
		} else {
			c.slice = c.slice[:start]
			num := start + n - cap(c.slice)
			for i := 0; i < num; i++ {
				c.ch <- c.diskSlice[i]
			}
			copy(c.diskSlice[0:], c.diskSlice[num:])
			c.diskSlice = c.diskSlice[:len(c.diskSlice)-num]
		}

		n = min(cap(c.slice)-len(c.slice), len(c.diskSlice))
		for i := 0; i < n; i++ {
			t, err := c.read(fmt.Sprintf("%d", c.diskSlice[i]))
			if err != nil {
				return fmt.Errorf(GetError, err.Error())
			}
			c.slice = append(c.slice, t)
			c.ch <- c.diskSlice[i]
		}
		if n > 0 {
			copy(c.diskSlice[0:], c.diskSlice[n:])
			c.diskSlice = c.diskSlice[:len(c.diskSlice)-n]
		}
		return nil
	}

	for i := start - cap(c.slice); i < start-cap(c.slice)+n; i++ {
		c.ch <- c.diskSlice[i]
	}
	copy(c.diskSlice[start-cap(c.slice):], c.diskSlice[start-cap(c.slice)+n:])
	c.diskSlice = c.diskSlice[:len(c.diskSlice)-n]
	return nil
}

func (c *config[T]) Cleanup() {
	c.ch <- CLEANUP
}
