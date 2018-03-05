package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/carlosmecha/todo/server"
	"github.com/carlosmecha/todo/store"
)

func main() {

	bucket := flag.String("bucket", "cmecha-cloud", "S3 bucket")
	key := flag.String("key", "todo.md", "S3 key")
	region := flag.String("region", "us-west-2", "S3 region")
	port := flag.Int("port", 80, "HTTP port")

	flag.Parse()

	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Printf("Starting server in port %d", *port)

	s := store.NewStore(*bucket, *key, *region, logger)
	http := server.RunServer(fmt.Sprintf("0.0.0.0:%d", *port), s, logger)

	stop := make(chan os.Signal, 1)
	defer close(stop)
	signal.Notify(stop, os.Interrupt)
	<-stop

	http.Shutdown(context.Background())
	logger.Print("Server stopped")
}
