package handler

import (
	"log"
	"net/http"
)

func StartServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/upload", handleUpload)
	mux.HandleFunc("/download", handleDownload)
	mux.HandleFunc("/delete", handleDelete)

	log.Println("Starting server on port: 8080")
	log.Fatal(http.ListenAndServe(":8080", limitMiddleware(mux)))
}
