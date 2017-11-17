package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/inverse-inc/packetfence/go/log"
	"github.com/inverse-inc/packetfence/go/pfconfigdriver"
)

func DbFromConfig(ctx context.Context) (*sql.DB, error) {

	pfconfigdriver.PfconfigPool.AddStruct(ctx, &pfconfigdriver.Config.PfConf.Database)

	dbConfig := pfconfigdriver.Config.PfConf.Database

	return ConnectDb(ctx, dbConfig.User, dbConfig.Pass, dbConfig.Host, dbConfig.Db)
}

func ConnectDb(ctx context.Context, user, pass, host, dbName string) (*sql.DB, error) {
	proto := "tcp"
	if host == "localhost" {
		proto = "unix"
	}

	uri := fmt.Sprintf("%s:%s@%s(%s)/%s?parseTime=true", user, pass, proto, host, dbName)

	db, err := sql.Open("mysql", uri)

	if err != nil {
		log.LoggerWContext(ctx).Error(fmt.Sprintf("Error while connecting to DB: %s", err))
		return nil, err
	} else {
		return db, nil
	}
}
