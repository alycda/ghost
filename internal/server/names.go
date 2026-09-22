package server

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// A Ghost database gets a memorable name when the client does not choose one.
// The name is only a label: the Postgres database is named after the ID.

var adjectives = []string{
	"amber", "brisk", "calm", "clever", "daring", "eager", "fabled", "gentle",
	"hidden", "ivory", "jolly", "keen", "lucid", "mellow", "nimble", "opal",
	"plucky", "quiet", "rustic", "silver", "tidy", "umber", "vivid", "witty",
}

var animals = []string{
	"badger", "beaver", "bison", "crane", "dingo", "egret", "ferret", "gecko",
	"heron", "ibis", "jackal", "koala", "lemur", "marmot", "newt", "otter",
	"pika", "quail", "raven", "stoat", "tapir", "urchin", "vole", "wombat",
}

func randomName() (string, error) {
	a, err := rand.Int(rand.Reader, big.NewInt(int64(len(adjectives))))
	if err != nil {
		return "", fmt.Errorf("generating name: %w", err)
	}
	b, err := rand.Int(rand.Reader, big.NewInt(int64(len(animals))))
	if err != nil {
		return "", fmt.Errorf("generating name: %w", err)
	}
	return adjectives[a.Int64()] + "-" + animals[b.Int64()], nil
}
