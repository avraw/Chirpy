package main

import (
"fmt"
"net/http"
)
func main(){


mux := http.NewServeMux()

fh := http.FileServer()

dh := http.Dir(".")

mux.Handle("/",fh(dh))
s := http.Server{

Addr : ":8080",
Handler : mux,

}

s.ListenAndServe()
fmt.Println("Hello world")

}


