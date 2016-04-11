package main

import (
	"github.com/asdine/storm"
	"github.com/robfig/cron"
)

func createCronJob(database *storm.DB, command func()) {
	scheduler := cron.New()
	scheduler.AddFunc("0 30 14 * * *", command)
	scheduler.Start()
}
