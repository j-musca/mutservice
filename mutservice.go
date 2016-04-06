package main

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/buaazp/fasthttprouter"
	"github.com/nu7hatch/gouuid"
	"github.com/valyala/fasthttp"
	"log"
	"os"
	"time"
)

func main() {
	db, boltErr := bolt.Open(os.Getenv("HOME")+"/app-mut.db", 0600, &bolt.Options{Timeout: 1 * time.Second})

	if boltErr != nil {
		log.Fatal(boltErr)
	}

	defer db.Close()

	db.Update(createBuckets)

	serverError := fasthttp.ListenAndServe(":8081", createRouter(db).Handler)

	if serverError != nil {
		log.Fatalf("Error in ListenAndServe: %s", serverError)
	}
}

func createBuckets(tx *bolt.Tx) error {
	bucketNames := [3]string{"users", "keys", "moods"}

	for _, element := range bucketNames {
		_, err := tx.CreateBucketIfNotExists([]byte(element))

		if err != nil {
			return fmt.Errorf("Could not create bucket: %s", err)
		}
	}

	return nil
}

func createRouter(db *bolt.DB) (router *fasthttprouter.Router) {
	router = fasthttprouter.New()

	router.GET("/users", getUsersRequest(db))
	router.GET("/users/:uuid", getUserByUuidRequest(db))
	router.POST("/users", putUserRequest(db))

	return router
}

func getUsersRequest(db *bolt.DB) fasthttprouter.Handle {
	return (func(ctx *fasthttp.RequestCtx, params fasthttprouter.Params) {
		users, dbError := getUsers(db)

		if dbError != nil {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			ctx.SetBody([]byte(dbError.Error()))
		} else {
			usersResponse := UsersResponse{users}
			json, jsonError := json.Marshal(usersResponse)

			if jsonError != nil {
				ctx.SetStatusCode(fasthttp.StatusInternalServerError)
				ctx.SetBody([]byte(jsonError.Error()))
			} else {
				ctx.SetStatusCode(fasthttp.StatusOK)
				ctx.SetContentType("application/json")
				ctx.SetBody(json)
			}

		}
	})
}

func getUsers(db *bolt.DB) (users []User, dbError error) {
	dbError = db.View(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket([]byte("users"))
		usersBucket.ForEach(func(uuid []byte, email []byte) error {
			users = append(users, User{string(uuid), string(email)})
			return nil
		})
		return nil
	})

	return users, dbError
}

func getUserByUuidRequest(db *bolt.DB) fasthttprouter.Handle {
	return (func(ctx *fasthttp.RequestCtx, params fasthttprouter.Params) {
		uuid := params.ByName("uuid")
		user, dbError := getUserByUuid(db, uuid)

		if dbError != nil {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			ctx.SetBody([]byte(dbError.Error()))
		} else {
			if user != nil {
				json, jsonError := json.Marshal(user)

				if jsonError != nil {
					ctx.SetStatusCode(fasthttp.StatusInternalServerError)
					ctx.SetBody([]byte(jsonError.Error()))
				} else {
					ctx.SetStatusCode(fasthttp.StatusOK)
					ctx.SetContentType("application/json")
					ctx.SetBody(json)
				}
			} else {
				ctx.SetStatusCode(fasthttp.StatusNotFound)
				ctx.SetBody([]byte("{\"message\":\"User with uuid '" + uuid + "' not found!\"}"))
			}
		}
	})
}

func getUserByUuid(db *bolt.DB, uuid string) (user *User, dbError error) {
	dbError = db.View(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket([]byte("users"))

		email := usersBucket.Get([]byte(uuid))

		if email != nil {
			user = &User{uuid, string(email)}
		}

		return nil
	})

	return user, dbError
}

func putUserRequest(db *bolt.DB) fasthttprouter.Handle {
	return (func(ctx *fasthttp.RequestCtx, params fasthttprouter.Params) {
		userCreation := UserCreation{}
		jsonError := json.Unmarshal(ctx.PostBody(), &userCreation)

		if jsonError != nil {
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			ctx.SetBody([]byte(jsonError.Error()))
		} else {
			user, dbError := saveUser(db, userCreation)

			fmt.Printf("Saved user: %s\n", user)

			if dbError != nil {
				ctx.SetStatusCode(fasthttp.StatusInternalServerError)
				ctx.SetBody([]byte(dbError.Error()))
			} else {
				json, _ := json.Marshal(user)
				ctx.SetStatusCode(fasthttp.StatusCreated)
				ctx.SetContentType("application/json")
				ctx.SetBody(json)
			}

		}

	})
}

func saveUser(db *bolt.DB, userCreation UserCreation) (user User, err error) {
	uuid, _ := uuid.NewV4()

	dbError := db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("users"))
		return bucket.Put([]byte(uuid.String()), []byte(userCreation.Email))
	})

	return User{uuid.String(), userCreation.Email}, dbError
}

type User struct {
	Uuid  string `json:"uuid"`
	Email string `json:"email"`
}

type UsersResponse struct {
	Users []User `json:"users"`
}

type UserCreation struct {
	Email string `json:"email"`
}
