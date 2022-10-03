package main

import (
	"bytes"
	"crypto/sha512"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/streadway/amqp"
	// amqp "github.com/rabbitmq/amqp091-go"
)

type Page struct {
	Id              int
	Url             string
	Last_crawl_time sql.NullInt64
	Crawl_count     sql.NullInt64
	File            sql.NullString
	New_pages       sql.NullInt64
	Found_from      int
}

func main() {
	amqp_conn, err := amqp.Dial("amqp://user:password@rabbitmq:5672/")
	checkErr("AMQP Connection", err)

	ch, err := amqp_conn.Channel()
	checkErr("AMQP Channel", err)
	defer ch.Close()

	// err = ch.Qos(
	// 	100,   // prefetch count
	// 	0,     // prefetch size
	// 	false, // global
	// )
	// checkErr("Failed to set QoS", err)

	msgs, err := ch.Consume(
		"seeder", //queue
		"",       //consumer
		false,    //auto-ack
		false,    //exclusive
		false,    //no-local
		false,    //no-wait
		nil,      //args
	)
	checkErr("AMQP Consume", err)

	pubch, err := amqp_conn.Channel()
	checkErr("AMQP Pub Channel", err)
	defer pubch.Close()

	_, err = pubch.QueueDeclare(
		"extractor",
		false,
		false,
		false,
		false,
		nil,
	)
	// fmt.Println("Publish Queue: ", pubqueue)
	checkErr("AMQP extractor Queue", err)

	fmt.Println("Successfully Connected to our RabbitMQ Instance")
	fmt.Println(" [*] - Waiting for messages")

	var wg sync.WaitGroup

	//limit number of go routine
	maxGoroutines := 50
	guard := make(chan struct{}, maxGoroutines)

	//error rate
	// errorRate := 0.00
	errs := 1
	okays := 1

	//time mesaurement
	start := time.Now()

	for d := range msgs {
		var pg Page
		err := json.Unmarshal(d.Body, &pg)
		checkErr("JSON Unmarshal", err)
		wg.Add(1)

		//limit number of go routine
		guard <- struct{}{} // would block if guard channel is already filled

		go func(delivery amqp.Delivery, pg Page) {
			defer wg.Done()
			defer func() {
				<-guard
			}()
			fmt.Println((100*errs)/(okays+errs), errs, okays)
			fmt.Println(int(time.Since(start).Milliseconds()) / (okays + errs))
			name, err := crawlAndSave(pg)
			if err != nil {
				delivery.Nack(false, true)
				checkErr("Crawl and save", err)
				errs += 1
				return
			}
			pg.File.String = name
			msg := new(bytes.Buffer)
			json.NewEncoder(msg).Encode(pg)

			//ack message
			delivery.Ack(false)

			err = pubch.Publish(
				"",
				"extractor",
				false,
				false,
				amqp.Publishing{
					ContentType: "text/plain",
					Body:        msg.Bytes(),
				},
			)

			checkErr("AMQP extractor Publish", err)
			okays += 1
			// fmt.Println("DONE: ", pg.Url, string(msg.Bytes()))
		}(d, pg)
	}

	wg.Wait()
}

func crawlAndSave(pg Page) (string, error) {

	var filepath string

	// decenaryID, err := nanoid.CustomASCII("0123456789AaBbCcDdEeFfGgHhIiJjKkLlMmNnOoPpQqRrSsTtUuVvWwXxYyZz", 21*2)
	// checkErr("Nanoid", err)

	s := sha512.New()
	s.Write([]byte(pg.Url))
	tmpName := hex.EncodeToString(s.Sum(nil))
	if len(pg.File.String) == 0 {
		filepath = "/data/" + tmpName
	} else {
		filepath = "/data/" + pg.File.String
	}

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(pg.Url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	return tmpName, nil
}

func checkErr(txt string, err error) {
	red := string("\033[0;31m")
	reset := string("\033[0m")
	if err != nil {
		fmt.Println(time.Now(), red, txt+": "+err.Error(), reset)
	}
}
