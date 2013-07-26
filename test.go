package main

import "fmt"
func main() {
  // #=> slice is pass by reference
  test := []int{1, 2}
  fmt.Println(test)
  modify(test)
  fmt.Println(test)

  fmt.Println()
  fmt.Println()

  // #=> append() returns the updated slice
  // #=> therefore it is necessary to store the result of append?
  test_append := []int{1, 2}
  fmt.Println(test_append)
  test_append = modify_append(test_append)
  fmt.Println(test_append)
}

func modify(slice []int) {
  slice[0] = 3
  slice[1] = 4
  return
}

func modify_append(slice[] int) []int {
  another := []int{3,4}
  for _, item := range another {
    slice = append(slice, item)  
  }
  
  return slice
}

// git:(master) ✗ go run test.go
// [1 2]
// [3 4]


// [1 2]
// [1 2]