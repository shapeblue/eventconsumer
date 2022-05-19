package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/streadway/amqp"
	"github.com/melbahja/goph"
)

is_debug := false
router_ip := "10.0.112.134"

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

func executeIpv6RouteCmdOnRouter(cmd string) {
	client, err := goph.New("root", router_ip, goph.Password("P@ssword123"))
	if err != nil {
		log.Printf("Failed to connect to %s, %v", router_ip, err)
	}
	// Defer closing the network connection.
	defer client.Close()
	// Execute your command.
	out, err := client.Run(cmd)
	if err != nil {
		log.Printf("Failed to run command %s on %s, %v", cmd, router_ip, err)
	}
	// Get your output as []byte.
	log.Printf(string(out))
}

func processIpv6Event(event Event) {
	log.Printf("Parsed Event--> Type:: %s, Status:: %s, Description:: %s", event.Event, event.Status, event.Description)

	if (strings.Contains(event.Description, "Subnet: ") && strings.Contains(event.Description, ", gateway: ")) {
		route := strings.Split(event.Description, "Subnet: ")[1]
		routelist := strings.Split(route, ", gateway: ")
		subnet := routelist[0]
		gateway := routelist[1]
		log.Printf("Static route--> Subnet:: %s, Gateway:: %s", subnet, gateway)
		operation := "add"
		if (event.Event == "NET.IP6RELEASE") {
			operation = "delete"
		}
		cmd := fmt.Sprintf("ip -6 route %s %s via %s", operation, subnet, gateway)
		log.Printf("Cmd--> %s", cmd)
		executeIpv6RouteCmdOnRouter(cmd)
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
		if is_debug == true {
			log.Printf("--------------START----------------")
			log.Printf("Incoming: %s", string(d.Body))
		}
		var event Event

		err := json.Unmarshal(d.Body, &event)
		if is_debug == true {
			log.Println("Event: %s, %s", event, event.Event)
		}

		if err == nil && (event.Event == "NET.IP6ASSIGN" || event.Event == "NET.IP6RELEASE") && event.Status == "Completed" && event.Description != "" {
			processIpv6Event(event)
		}
		erro := d.Ack(true)
		if erro != nil {
			log.Printf("Ack error: %s", erro)
		}
		if is_debug == true {
			log.Printf("---------------END-----------------")
		}
	}

}
