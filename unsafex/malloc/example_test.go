package malloc

import "fmt"

func Example() {
	arena := make([]byte, 512*1024)
	a, _ := NewBuddyAllocator(arena)

	b1 := a.Alloc(1024) // fits in 8KB block
	b2 := a.Alloc(8192) // needs 16KB block due to 8-byte header

	fmt.Printf("b1: len=%d cap=%d\n", len(b1), cap(b1))
	fmt.Printf("b2: len=%d cap=%d\n", len(b2), cap(b2))

	a.Free(b1)
	a.Free(b2)

	// Output:
	// b1: len=1024 cap=8184
	// b2: len=8192 cap=16376
}
