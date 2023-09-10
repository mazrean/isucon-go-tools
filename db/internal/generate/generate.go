package isudbgen

//go:generate sh -c "go run ../../../internal/tools/interface-wrapper/ < driver_driver.json > driver_driver.go"
//go:generate sh -c "go run ../../../internal/tools/interface-wrapper/ < driver_conn.json > driver_conn.go"
//go:generate sh -c "go run ../../../internal/tools/interface-wrapper/ < driver_stmt.json > driver_stmt.go"
