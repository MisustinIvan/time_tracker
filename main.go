package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db_dir string
var db *sql.DB

func initialize() error {
	err := os.MkdirAll(db_dir, os.ModePerm)
	if err != nil {
		return err
	}

	const query = `
create table time_entries (
id integer primary key autoincrement,
start datetime,
duration time,
description text
);
create table tax (
	rate real
);
insert into tax (rate) values (0);
create table wage (
	rate real
);
insert into wage (rate) values (0);
`

	_, err = db.Exec(query)
	return err
}

func set_tax(args ...string) (float64, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("Invalid amount of arguments")
	}

	rate, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return 0, fmt.Errorf("Failed to parse argument as number: %e\n", err)
	}

	query := "update tax set rate = ?"

	if _, err := db.Exec(query, rate); err != nil {
		return 0, err
	}

	return rate, nil
}

func set_wage(args ...string) (float64, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("Invalid amount of arugments")
	}

	rate, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return 0, fmt.Errorf("Failed to parse argument as number: %e\n", err)
	}

	query := "update wage set rate = ?"

	if _, err := db.Exec(query, rate); err != nil {
		return 0, err
	}

	return rate, nil
}

func add(args ...string) error {
	darg := args[0]
	var unit time.Duration
	switch darg[len(darg)-1] {
	case 'h':
		unit = time.Hour
	case 'm':
		unit = time.Minute
	case 's':
		unit = time.Second
	default:
		return fmt.Errorf("Duration with invalid unit: %s", darg)
	}

	n, err := strconv.ParseFloat(darg[:len(darg)-1], 64)
	if err != nil {
		return err
	}

	duration := time.Duration(float64(unit) * n)
	start_time := time.Now().Add(-duration)

	query := "insert into time_entries (duration, start, description) values (?,?,?)"
	description := strings.Join(args[1:], " ")
	_, err = db.Exec(query, duration, start_time, description)
	if err != nil {
		return err
	}

	return nil
}

func total(args ...string) (time.Duration, error) {
	if len(args) != 2 {
		return 0, fmt.Errorf("Invalid number of arguments!")
	}

	month_s := args[0]
	month_i, err := strconv.ParseInt(month_s, 10, 64)
	if err != nil {
		return 0, err
	}

	year_s := args[1]
	year_i, err := strconv.ParseInt(year_s, 10, 64)
	if err != nil {
		return 0, err
	}

	start := time.Date(int(year_i), time.Month(month_i), 0, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	query := "select coalesce(sum(duration), 0) from time_entries where start > ? and start < ?"
	var total time.Duration
	if err := db.QueryRow(query, start, end).Scan(&total); err != nil {
		return 0, err
	}

	return total, nil
}

func total_money(args ...string) (float64, error) {
	if len(args) != 2 {
		return 0, fmt.Errorf("Invalid number of arguments!")
	}

	month_s := args[0]
	month_i, err := strconv.ParseInt(month_s, 10, 64)
	if err != nil {
		return 0, err
	}

	year_s := args[1]
	year_i, err := strconv.ParseInt(year_s, 10, 64)
	if err != nil {
		return 0, err
	}

	start := time.Date(int(year_i), time.Month(month_i), 0, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	query := `
    select
        (coalesce(sum(duration), 0) / 3600000000000.0)
        * coalesce((select rate from wage limit 1), 0)
        * (1 - coalesce((select rate from tax limit 1), 0))
    from time_entries
    where start > ? and start < ?
`
	var money float64
	if err := db.QueryRow(query, start, end).Scan(&money); err != nil {
		return 0, err
	}

	return money, nil
}

func print_usage() {
	const usage = `
time_tracker - a tool for tracking time spent on various activities

usage: time_tracker {command} {arguments...}

commands:
	- add: {duration} {description}
	- total: {month} {year}
	- total_money: {month} {year}
	- set_tax: {rate}
	- set_wage: {rate}
	- init
`
	fmt.Printf(usage)
}

func setup() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	const db_location_home string = ".config/time_tracker/"

	db_dir = home + "/" + db_location_home

	db, err = sql.Open("sqlite3", db_dir+"/"+"db.db")
	return err
}

func cleanup() {
	db.Close()
}

func main() {
	if err := setup(); err != nil {
		fmt.Printf("Could not setup program: %s\n", err)
		os.Exit(-1)
	}

	if len(os.Args) < 2 {
		print_usage()
		os.Exit(-1)
	}

	switch os.Args[1] {
	case "add":
		if err := add(os.Args[2:]...); err != nil {
			fmt.Printf("Could not add a new entry: %s\n", err)
		} else {
			fmt.Printf("Successfully added entry\n")
		}
	case "total":
		if time, err := total(os.Args[2:]...); err != nil {
			fmt.Printf("Could not get total: %s\n", err)
		} else {
			fmt.Printf("Total: %s", time.String())
		}
	case "total_money":
		if money, err := total_money(os.Args[2:]...); err != nil {
			fmt.Printf("Could not get total: %s\n", err)
		} else {
			fmt.Printf("Total: %f Kč", money)
		}
	case "init":
		if err := initialize(); err != nil {
			fmt.Printf("Could not initialize database: %s\n", err)
		} else {
			fmt.Printf("Successfully initialized database\n")
		}
	case "set_tax":
		if rate, err := set_tax(os.Args[2:]...); err != nil {
			fmt.Printf("Failed to set tax rate: %e\n", err)
			return
		} else {
			fmt.Printf("Successfully set tax rate to: %f\n", rate)
			return
		}
	case "set_wage":
		if rate, err := set_wage(os.Args[2:]...); err != nil {
			fmt.Printf("Failed to set tax rate: %e\n", err)
			return
		} else {
			fmt.Printf("Successfully set tax rate to: %f\n", rate)
			return
		}
	default:
		print_usage()
	}

	cleanup()
}
