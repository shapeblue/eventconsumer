package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/streadway/amqp"
)

// Message =  {"id":"c7a49098-fef6-44ef-9311-e5e10ef6d487","eventDateTime":"2014-09-01 17:14:05 +0200","event":"VM.CREATE","resource":"com.cloud.vm.VirtualMachine","account":"ecbbc7ae-31e2-11e4-9822-005dd411d6fc","zone":"018a5248-9761-4363-946d-c765d07ac19e"}

type Event struct {
	Id       		string `json:"id"`
	Event    		string `json:"event"`
	EventDateTime   string `json:"eventDateTime"`
	Status    		string `json:"status"`
	Entity    		string `json:"entity"`
	Entityuuid 		string `json:"entityuuid"`
	Account  		string `json:"account"`
	User	  		string `json:"user"`
	Description		string `json:"description"`
}

func FailOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s:%s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

func consume(brokerUrl string, done chan bool) {
	conn, err := amqp.Dial(brokerUrl)
	FailOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	FailOnError(err, "Failed to open a channel")
	defer ch.Close()
	ch.Qos(1, 0, false)
	q, err := ch.QueueDeclare(
		"acsevents", // name
		true,        // durable
		false,       // delete when usused
		false,       // exclusive
		false,       // noWait
		nil,         // arguments
	)
	FailOnError(err, "Failed to declare a queue")

	err = ch.QueueBind(
		q.Name,              // queue name
		"#",                 // routing key
		"cloudstack-events", // exchange
		false,
		nil)
	FailOnError(err, "Failed to bind a queue")

	msgs, err := ch.Consume(q.Name, "go-event-eater", false, false, false, false, nil)

	if err != nil {
		log.Fatal("Consume error: %s", err)
	}
	var d amqp.Delivery

	log.Printf("Consuming now")
	for d = range msgs {
		// fmt.Println("\n\n--------------START----------------")
		// fmt.Println("Incoming:", string(d.Body))
		var event Event

		err := json.Unmarshal(d.Body, &event)
		// log.Println("Event:", event, event.Event)

		if err == nil && (event.Event == "IP6.ASSIGN" || event.Event == "IP6.RELEASE") && event.Status != "" && event.Description != ""  {
			// fmt.Println("Event Body:", string(d.Body))
			fmt.Printf("Parsed Event:: Type: %s, Status: %s, Description: %s\n\n", event.Event, event.Status, event.Description)
		}
		erro := d.Ack(true)
		if erro != nil {
			log.Printf("Ack error: %s", erro)
		}
		// fmt.Println("\n\n---------------END-----------------")
	}

}
