module github.com/somakeit/door-controller3

go 1.16

require (
	github.com/go-sql-driver/mysql v1.5.0
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	gopkg.in/DATA-DOG/go-sqlmock.v1 v1.3.0
	periph.io/x/conn/v3 v3.6.9
	periph.io/x/devices/v3 v3.6.13-0.20211029203041-00ed90382f0b
	periph.io/x/host/v3 v3.7.1
)

replace periph.io/x/devices/v3 => github.com/brackendawson/devices/v3 v3.6.8-0.20211110200218-3ecf30d05049
