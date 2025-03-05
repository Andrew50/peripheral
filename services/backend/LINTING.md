# Go Backend Linting Guide

## Current Status

The Go backend codebase has several golangci-lint issues that have been temporarily suppressed to allow CI/CD pipelines to pass. This document provides guidance on how to gradually fix these issues.

## Common Linting Issues

The most frequent issues are:

1. **Unchecked Error Returns (errcheck)**
   ```go
   // Bad
   socket.StartPolygonWS(conn, false)
   
   // Good
   err := socket.StartPolygonWS(conn, false)
   if err != nil {
       log.Printf("Failed to start Polygon WebSocket: %v", err)
       // Handle the error appropriately
   }
   ```

2. **Unused Code (unused)**
   - Remove unused functions, types, and variables
   - If needed for future use, add a comment explaining why it's kept: 
   ```go
   // Kept for future implementation of feature X
   func unusedFunction() {...}
   ```

3. **Code Simplification (gosimple)**
   - Replace `for { select {} }` with `for range`
   - Replace `for true {}` with `for {}`
   - Replace `fmt.Sprintf("%v", x)` with `fmt.Sprint(x)`

4. **Mutex Copying (govet)**
   - Avoid returning structs that contain mutexes
   - Use pointers to structs with mutexes
   ```go
   // Bad
   func GetData() TimeframeData {...}
   
   // Good
   func GetData() *TimeframeData {...}
   ```

5. **Ineffectual Assignments (ineffassign)**
   - Fix assignments where the value is never used
   ```go
   // Bad
   err := doSomething()
   err = doSomethingElse() // First error is lost
   
   // Good
   err := doSomething()
   if err != nil {
       return err
   }
   err = doSomethingElse()
   ```

6. **Deprecated Packages (staticcheck)**
   - Replace `io/ioutil` with `io` or `os` alternatives:
   ```go
   // Bad
   content, err := ioutil.ReadFile("file.txt")
   
   // Good
   content, err := os.ReadFile("file.txt")
   ```

## How to Fix Issues

1. **Run Local Linting**:
   ```bash
   cd backend
   golangci-lint run
   ```

2. **Fix Critical Issues First**:
   - Error checking
   - Mutex copying 
   - Ineffectual assignments

3. **Clean Up Gradually**:
   - Remove unused code
   - Simplify code
   - Update deprecated API usage

4. **Re-enable Linters One by One**:
   As issues are fixed, enable linters in `.golangci.yml`:
   ```yaml
   linters:
     disable:
       # - unused  # Re-enabled after fixing unused code
       - gosimple 
       - staticcheck
       # and so on
   ```

## Important Notes

1. **Always Test After Fixes**: Linting fixes can change behavior, especially when fixing error handling
2. **Handle Errors Appropriately**: Don't just discard errors; log them or handle them properly
3. **Consider Context**: Some "unused" code might be there for future use or debugging; add comments to explain

## Long-term Plan

1. **Set Up Pre-commit Hooks**: Prevent new issues from being introduced
2. **Enforce Code Reviews**: Ensure new code passes linting
3. **Regular Cleanup**: Schedule periodic cleanup sessions
4. **Update Dependencies**: Keep libraries and Go version up to date 