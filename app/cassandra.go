package app

import (
	"time"

	"github.com/gocql/gocql"
)

type CassandraConfig struct {
	Nodes       string  `yaml:"Nodes"`
	Consistency *string `yaml:"Consistency"`
}

func ConnectCassandra(config CassandraConfig) (*gocql.Session, error) {
	cluster := gocql.NewCluster(config.Nodes)
	cluster.Keyspace = "phoenix"
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	if config.Consistency != nil {
		cluster.Consistency = gocql.ParseConsistency(*config.Consistency)
	} else {
		cluster.Consistency = gocql.Quorum
	}
	return cluster.CreateSession()
}
