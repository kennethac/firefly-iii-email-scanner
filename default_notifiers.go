package main

import (
	"log"
)

type StdOutNotifier struct{}

func (s *StdOutNotifier) Notify(message string) error {
	log.Printf("StdOutNotifier: %s", message)
	return nil
}

type NoOpNotifier struct{}

func (n *NoOpNotifier) Notify(message string) error {
	return nil
}
