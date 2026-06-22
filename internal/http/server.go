package http_handler

import "log"
import "net/http"

func handleTest(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP] Request arrive for Test Path...")
	w.Write([]byte("This is the test path..."))
}

func Serve() {
	dir := "assets/http/"
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir(dir)))
	mux.HandleFunc("/test", handleTest)
	log.Fatal(http.ListenAndServe(":8033", mux))
}
