package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	// _ "github.com/mattn/go-sqlite3"
	_ "github.com/go-sql-driver/mysql"
	"github.com/streadway/amqp"
	// amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	amqp_conn, err := amqp.Dial("amqp://user:password@rabbitmq:5672/")
	checkErr("AMQP Connection", err)
	//defer amqp.Close()

	ch, err := amqp_conn.Channel()
	checkErr("AMQP Channel", err)
	defer ch.Close()

	queue, err := ch.QueueDeclare(
		"seeder",
		false,
		false,
		false,
		false,
		nil,
	)
	fmt.Println(queue)
	checkErr("AMQP Queue", err)

	// db, err := sql.Open("sqlite3", "../database/data.db")
	db, err := sql.Open("mysql", "root:password@tcp(db:3306)/crawler")
	checkErr("Database Open", err)
	//defer db.Close()

	rows, err := db.Query("SELECT * from (SELECT * FROM `pages` ORDER BY last_crawl_time) AS `t` ORDER BY crawl_count")
	checkErr("Database Query", err)
	// defer rows.Close()

	type Page struct {
		Id              int
		Url             string
		Last_crawl_time sql.NullInt64
		Crawl_count     sql.NullInt64
		File            sql.NullString
		New_pages       sql.NullInt64
		Found_from      int
	}

	var pages []Page

	for rows.Next() {
		var pg Page
		err := rows.Scan(&pg.Id, &pg.Url, &pg.Last_crawl_time, &pg.Crawl_count, &pg.File, &pg.New_pages, &pg.Found_from)
		if err != nil {
			fmt.Println(err.Error())
		}
		pages = append(pages, pg)
	}
	rows.Close()

	for _, pg := range pages {
		msg := new(bytes.Buffer)
		json.NewEncoder(msg).Encode(pg)

		err = ch.Publish(
			"",
			"seeder",
			false,
			false,
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        msg.Bytes(),
			},
		)
		checkErr("AMQP Publish", err)
	}

	amqp_conn.Close()
	db.Close()
	time.Sleep(time.Minute * 5)
	main()
}

func checkErr(txt string, err error) {
	red := string("\033[0;31m")
	reset := string("\033[0m")
	if err != nil {
		fmt.Println(time.Now(), red, txt+": "+err.Error(), reset)
	}
}
