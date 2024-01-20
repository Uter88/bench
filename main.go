package main

import (
	"context"
	"log"
	"os"
	"os/signal"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	b := NewBench()

	if err := b.ParseArgs(); err != nil {
		log.Fatalln(err)
	}
	go func() {
		<-ctx.Done()
		b.PrintResult()
		os.Exit(1)
	}()

	b.Run()
	b.PrintResult()
}
