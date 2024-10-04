package main

import (
	"log"
	"net/smtp"
	"os"
)

func main() {
	auth := smtp.PlainAuth("contact@alexshroyer.com", "ashroyer@gmail.com", os.Getenv("APP_PASSWORD"), "smtp.gmail.com")
	to := []string{"ashroyer@gmail.com"}
	msg := []byte("To: ashroyer@gmail.com\r\n" +
		"Subject: test email 2\r\n" +
		"\r\n" +
		"This is the email body.\r\n")
	err := smtp.SendMail("smtp.gmail.com:587", auth, "contact@alexshroyer.com", to, msg)
	if err != nil {
		log.Fatal(err)
	}

}
