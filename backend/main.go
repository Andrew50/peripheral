package main


import (
    "os"
    "fmt"
    "api/server"
)


func main() {
    args := os.Args
    if len(args) > 1 {
        fmt.Println("test")
        //test func
    } else {
        server.StartServer()
    }

}

