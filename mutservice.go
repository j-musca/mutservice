package main

import (
	"github.com/asdine/storm"
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine/fasthttp"
	"github.com/labstack/echo/middleware"
	"log"
	"net/http"
)

type (
	Mood struct {
		Value int `json:"mood"`
	}

	Subscribers struct {
		Users []Subscriber `json:"users"`
	}

	Subscription struct {
		Email string `json:"email"`
	}
)

func main() {
	database := createDatabase()
	defer database.Close()

	createCronJob(database, triggerMail(database))

	server := initServer(database)
	server.Run(fasthttp.New(":8081"))

	log.Println("Started server on port 8081.")
}

func initServer(database *storm.DB) (server *echo.Echo) {
	server = echo.New()

	server.Use(middleware.Logger())
	server.Get("/subscribers", getSubscribers(database))
	server.Get("/subscribers/:uuid", getSubscribersByUuid(database))
	server.Post("/subscribers", postSubscriber(database))
	server.Get("/moods/:key", getDailyMoods())
	server.Post("/moods/:key", postDailyMoods(database))

	return server
}

func getDailyMoods() echo.HandlerFunc {
	return (func(context echo.Context) error {
		key := context.Param("key")

		htmlContent := `<html>
	<body>
	<h1>Select your mood</h1>
	<form method="POST" action="http://aulendorf:8081/moods/` + key + `">
	<input type="hidden" name="mood" value="0">
	<input type="submit" value="Very unhappy">
	</form>
	<br/>
	<form method="POST" action="http://aulendorf:8081/moods/` + key + `">
	<input type="hidden" name="mood" value="1">
	<input type="submit" value="Unhappy">
	</form>
	<br/>
	<form method="POST" action="http://aulendorf:8081/moods/` + key + `">
	<input type="hidden" name="mood" value="2">
	<input type="submit" value="Neutral">
	</form>
	<br/>
	<form method="POST" action="http://aulendorf:8081/moods/` + key + `">
	<input type="hidden" name="mood" value="3">
	<input type="submit" value="Happy">
	</form>
	<br/>
	<form method="POST" action="http://aulendorf:8081/moods/` + key + `">
	<input type="hidden" name="mood" value="4">
	<input type="submit" value="Very happy">
	</form>
	</body>
	</html>`
		return context.HTML(http.StatusOK, htmlContent)

	})
}

func postDailyMoods(database *storm.DB) echo.HandlerFunc {
	return (func(context echo.Context) error {
		key := context.Param("key")
		mood := context.FormValue("mood")

		if feedbackIdentifier := getFeedbackIdentifier(database, key); feedbackIdentifier != nil {
			if databaseError := updateDailyMoods(database, feedbackIdentifier.DateString, mood); databaseError != nil {
				return databaseError
			} else {
				return context.String(http.StatusCreated, "Thank you!")
			}
		} else {
			return context.String(http.StatusNotFound, "Mood with key '"+key+"' not found!")
		}

		return nil
	})
}

func getSubscribers(database *storm.DB) echo.HandlerFunc {
	return (func(context echo.Context) error {
		subscriptions, databaseError := getSubscriptions(database)

		if databaseError != nil {
			return databaseError
		} else {
			return context.JSON(http.StatusOK, subscriptions)
		}

		return nil
	})
}

func getSubscribersByUuid(database *storm.DB) echo.HandlerFunc {
	return (func(context echo.Context) error {
		uuid := context.Param("uuid")
		subscriber, databaseError := getSubscriptionByUuid(database, uuid)

		if databaseError != nil {
			return databaseError
		} else {
			if subscriber != nil {
				return context.JSON(http.StatusOK, subscriber)
			} else {
				return context.String(http.StatusNotFound, "User with uuid '"+uuid+"' not found!")
			}
		}

		return nil
	})
}

func postSubscriber(db *storm.DB) echo.HandlerFunc {
	return (func(context echo.Context) error {
		subscription := new(Subscription)
		if jsonError := context.Bind(subscription); jsonError != nil {
			return jsonError
		} else {
			subscriber, databaseError := saveSubscription(db, subscription)

			log.Printf("Saved user: %s\n", subscriber)

			if databaseError != nil {
				return databaseError.Error()
			} else {
				return context.JSON(http.StatusCreated, subscriber)
			}
		}

		return nil
	})
}
