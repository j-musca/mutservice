package main

import (
	"crypto/sha1"
	"encoding/hex"
	"github.com/asdine/storm"
	"github.com/nu7hatch/gouuid"
	"log"
	"os"
	"strings"
	"time"
)

type (
	FeedbackIdentifier struct {
		Key        string `storm:"id"`
		DateString string `storm:"index"`
	}

	DailyMoods struct {
		DateString  string `storm:"id"`
		VeryUnhappy int
		Unhappy     int
		Neutral     int
		Happy       int
		VeryHappy   int
	}

	Subscriber struct {
		Uuid  string `json:"uuid" storm:"id"`
		Email string `json:"email" storm:"unique"`
	}
)

func (dailyMoods *DailyMoods) AddMood(mood string) {
	if mood == "0" {
		dailyMoods.VeryUnhappy++
	}
	if mood == "1" {
		dailyMoods.Unhappy++
	}
	if mood == "2" {
		dailyMoods.Neutral++
	}
	if mood == "3" {
		dailyMoods.Happy++
	}
	if mood == "4" {
		dailyMoods.VeryHappy++
	}
}

func createDatabase() (database *storm.DB) {
	database, databaseError := storm.Open(getDataDirectory() + "app-mut.db")

	if databaseError != nil {
		log.Fatal(databaseError)
	}

	_ = database.Init(&Subscriber{})
	_ = database.Init(&FeedbackIdentifier{})
	_ = database.Init(&DailyMoods{})

	return database
}

func getDataDirectory() string {
	if os.Getenv("OPENSHIFT_DATA_DIR") != "" {
		return os.Getenv("OPENSHIFT_DATA_DIR")
	} else {
		return os.Getenv("HOME") + "/"
	}
}

func saveDailyMoods(database *storm.DB, dateString string) (databaseError error) {
	dailyMoods := new(DailyMoods)
	dailyMoods.DateString = dateString
	return database.Save(dailyMoods)
}

func updateDailyMoods(database *storm.DB, dateString string, mood string) (databaseError error) {
	dailyMoods := new(DailyMoods)
	databaseError = database.One("DateString", dateString, dailyMoods)

	if databaseError != nil {
		return databaseError
	}

	dailyMoods.AddMood(mood)

	return database.Save(dailyMoods)
}

func saveSubscriber(database *storm.DB, subscription *Subscription) (subscriber Subscriber, databaseError error) {
	uuid, _ := uuid.NewV4()
	subscriber = Subscriber{uuid.String(), subscription.Email}
	databaseError = database.Save(&subscriber)

	return subscriber, databaseError
}

func getSubscriberByUuid(database *storm.DB, uuid string) (subscriber *Subscriber, databaseError error) {
	subscriber = new(Subscriber)
	databaseError = database.One("Uuid", uuid, subscriber)
	return subscriber, databaseError
}

func getAllSubscribers(database *storm.DB) (subscribers []Subscriber, databaseError error) {
	databaseError = database.All(&subscribers)
	return subscribers, databaseError
}

func getFeedbackIdentifier(database *storm.DB, key string) (feedbackIdentifier *FeedbackIdentifier) {
	feedbackIdentifier = new(FeedbackIdentifier)
	databaseError := database.One("Key", key, feedbackIdentifier)

	if databaseError != nil {
		return nil
	}

	database.Remove(feedbackIdentifier)

	return feedbackIdentifier
}

func saveFeedbackIdentifierAndCreateMailTasks(subscribers []Subscriber, database *storm.DB) (tasks []MailTask, databaseError error) {
	today := time.Now().Format("02-01-2006")

	if databaseError = saveDailyMoods(database, today); databaseError != nil {
		return nil, databaseError
	}

	for _, subscriber := range subscribers {
		key := createKey(subscriber.Uuid, today)
		feedbackIdentifier := FeedbackIdentifier{key, today}
		databaseError = database.Save(&feedbackIdentifier)

		if databaseError != nil {
			return nil, databaseError
		}

		tasks = append(tasks, MailTask{subscriber.Email, key})
	}

	return tasks, databaseError
}

func createKey(uuid string, dateString string) (key string) {
	source := strings.Join([]string{uuid, dateString}, "-")
	hashCreator := sha1.New()
	hashCreator.Write([]byte(source))
	key = hex.EncodeToString(hashCreator.Sum(nil))

	return key
}
