package main

import (
	"github.com/asdine/storm"
	"github.com/ddliu/go-httpclient"
	"log"
	"os"
)

type (
	MailTask struct {
		Email string
		Key   string
	}
)

const BasicAuthHeader string = "Basic " + os.Getenv("MUT_BASIC_AUTH")
const MailGunUrl string = os.Getenv("MUT_MAILGUN_URL")

func sendMail(email string, text string) {
	response, responseError := httpclient.WithHeader("Authorization", BasicAuthHeader).Post(MailGunUrl,
		map[string]string{
			"from":    "Mailgun Sandbox <postmaster@sandbox4ebeef9e81ca4130885ef51fa4b9729f.mailgun.org>",
			"to":      email,
			"subject": "How is your mood today?",
			"html":    text,
		})
	if responseError != nil {
		log.Printf("%s", responseError)
	}

	defer response.Body.Close()
}

func triggerMail(database *storm.DB) func() {
	return func() {
		log.Println("Triggered mail sending!")
		subscriptions, triggerError := getSubscriptions(database)

		if triggerError != nil {
			log.Printf("%s", triggerError)
		}

		mailTasks, triggerError := saveFeedbackIdentifierAndCreateMailTasks(subscriptions, database)

		if triggerError != nil {
			log.Printf("%s", triggerError)
		}

		sendMails(mailTasks)
	}
}

func sendMails(tasks []MailTask) {
	for _, task := range tasks {
		sendMail(task.Email, getHtmlText(task.Key))
	}
}

func getHtmlText(key string) string {
	return `<html>
	<body>
	<h1>Select your mood</h1>
	<a href="http://aulendorf:8081/moods/` + key + `">Take me to the mood selection!</a>
	</body>
	</html>`
}
