package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/andrew-d/go-termutil"
	"github.com/codegangsta/cli"
	"github.com/streadway/amqp"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "pipecat"
	app.Usage = "Connect unix pipes and message queues"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "amqpUri",
			Value:  "amqp://guest:guest@localhost:5672/",
			Usage:  "AMQP URI",
			EnvVar: "AMQP_URI",
		},
	}

	app.Action = func(c *cli.Context) {
		list := c.Args().First()
		if list == "" {
			fmt.Println("Please provide name of the queue")
			os.Exit(1)
		}

		conn, err := amqp.Dial(c.String("amqpUri"))
		failOnError(err, "Failed to connect to AMPQ broker")
		defer conn.Close()

		channel, err := conn.Channel()
		failOnError(err, "Failed to open a channel")
		defer channel.Close()

		q, err := channel.QueueDeclare(
			list,  // name
			true,  // durable
			false, // delete when unused
			false, // exclusive
			false, // no-wait
			nil,   // arguments
		)
		failOnError(err, "Failed to declare a queue")

		unackedMessages := make(map[string]amqp.Delivery)
		if termutil.Isatty(os.Stdin.Fd()) {
			msgs, err := channel.Consume(
				q.Name, // queue
				"",     // consumer
				false,  // auto-ack
				false,  // exclusive
				false,  // no-local
				false,  // no-wait
				nil,    // args
			)
			failOnError(err, "Failed to register a consumer")
			for msg := range msgs {
				line := fmt.Sprintf("%s", msg.Body)
				unackedMessages[line] = msg
				fmt.Println(line)
				fmt.Fprintln(os.Stderr, fmt.Sprintf("%d", len(unackedMessages)))
			}
		} else {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				line := scanner.Text()
				err = channel.Publish(
					"",     // exchange
					q.Name, // routing key
					false,  // mandatory
					false,  // immediate
					amqp.Publishing{
						ContentType: "text/plain",
						Body:        []byte(line),
					})
				fmt.Println(line)
				failOnError(err, "Failed to publish a message")
			}
			if err := scanner.Err(); err != nil {
				fmt.Fprintln(os.Stderr, "Reading standard input:", err)
			}
		}
	}

	app.Run(os.Args)
}