package register

import (
	"crypto/rand"
	"math/big"
	"strconv"
	"time"
)

var firstNames = []string{
	"Emma", "Liam", "Olivia", "Noah", "Ava", "Elijah", "Sophia", "Lucas",
	"Mia", "Mason", "Isabella", "Logan", "Amelia", "Ethan", "Harper", "James",
	"Evelyn", "Benjamin", "Abigail", "Henry", "Ella", "Alexander", "Scarlett", "Daniel",
}

var lastNames = []string{
	"Smith", "Johnson", "Williams", "Brown", "Jones", "Miller", "Davis", "Garcia",
	"Rodriguez", "Wilson", "Martinez", "Anderson", "Taylor", "Thomas", "Moore", "Martin",
	"Lee", "Perez", "Thompson", "White", "Harris", "Sanchez", "Clark", "Lewis",
}

func randomProfile() (string, string) {
	name := firstNames[randomIndex(len(firstNames))] + " " + lastNames[randomIndex(len(lastNames))]
	now := time.Now().UTC()
	age := 22 + randomIndex(9)
	year := now.Year() - age
	month := time.Month(1 + randomIndex(12))
	day := 1 + randomIndex(27)
	return name, strconv.Itoa(year) + "-" + twoDigits(int(month)) + "-" + twoDigits(day)
}

func randomIndex(n int) int {
	if n <= 0 {
		return 0
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return int(time.Now().UnixNano() % int64(n))
	}
	return int(v.Int64())
}

func twoDigits(v int) string {
	if v < 10 {
		return "0" + strconv.Itoa(v)
	}
	return strconv.Itoa(v)
}
