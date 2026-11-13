package main

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

func main() {
	db, err := sql.Open("postgres", "postgres://nobody:nope@127.0.0.1:1/nodb")
	fmt.Println("Open err:", err)
	fmt.Println("Ping err:", db.Ping())
}
