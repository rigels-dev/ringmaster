# BRIEF: Todo CLI Application

## Objective
Build a lightweight, reliable CLI-based Todo Tracker application in Go.

## Requirements
1. `todo.go`:
   - Data structure: `TodoItem` (ID int, Title string, Completed bool).
   - Core functions:
     - `Add(title string) (*TodoItem, error)`
     - `Complete(id int) error`
     - `List() []TodoItem`
     - `Delete(id int) error`
2. `todo_test.go`:
   - Unit tests covering adding, completing, listing, and deleting items.
   - Test edge cases (e.g. deleting or completing non-existent ID).

## Acceptance Criteria
- [ ] `go test -v ./...` passes with 100% success.
- [ ] Code is well-structured and properly formatted.
