package main

import (
	"github.com/asdine/storm"
	"github.com/robfig/cron"
)

func createCronJob(dataBase *storm.DB, cmd func()) {
	scheduler := cron.New()
	scheduler.AddFunc("0 28 13 * * *", cmd)
	scheduler.Start()
}
