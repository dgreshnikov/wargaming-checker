package resultmanager

import (
	"wargaming-checker/wargaming"
)

type Status string

const (
	Bad           Status = "bad"
	Good          Status = "good"
	Error         Status = "error"
	MergeRequired Status = "merge_required"
)

type Result struct {
	Email     string
	Password  string
	Status    Status
	GameRealm string
	Vehicles  []wargaming.VehicleData
	Error     error
}

func NewGoodResult(email, password, gameRealm string, vehicles []wargaming.VehicleData) Result {
	return Result{
		Email:     email,
		Password:  password,
		GameRealm: gameRealm,
		Vehicles:  vehicles,
		Status:    Good,
	}
}

func NewBadResult(email, password string) Result {
	return Result{
		Email:    email,
		Password: password,
		Status:   Bad,
	}
}

func NewMergeRequiredResult(email, password string) Result {
	return Result{
		Email:    email,
		Password: password,
		Status:   MergeRequired,
	}
}

func NewErrorResult(email, password string, err error) Result {
	return Result{
		Email:    email,
		Password: password,
		Status:   Error,
		Error:    err,
	}
}
