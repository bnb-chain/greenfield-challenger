package dao

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Database is the struct for database docker
type Database struct {
	Name string
	DB   *gorm.DB

	pool     *dockertest.Pool
	resource *dockertest.Resource
	host     string
	port     string
	isGithub bool
}

// RunDB run docker of db for unit test
func RunDB(dbName string) (*Database, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, err
	}
	resource := &dockertest.Resource{}

	host := "mysql"
	port := "3306"

	_, isGithub := os.LookupEnv("GITHUB_ENV")
	if !isGithub {
		opt := docker.ListContainersOptions{
			All: true,
			Filters: map[string][]string{
				"ancestor": {"mysql:5.7"},
				"name":     {"challenger_unittest"},
			},
		}
		allContainers, err := pool.Client.ListContainers(opt)
		if err != nil {
			return nil, err
		}

		_, reuse := os.LookupEnv("REUSE_DOCKER")
		if !reuse || len(allContainers) == 0 {
			resource, err = pool.RunWithOptions(
				&dockertest.RunOptions{Repository: "mysql", Tag: "5.7", Env: []string{"MYSQL_ROOT_PASSWORD=root"}, Name: "challenger_unittest"},
			)
			if err != nil {
				return nil, err
			}
		} else {
			container := allContainers[0]
			if container.State != "running" {
				fmt.Printf("Try start non-running mysql docker\n")
				err := pool.Client.StartContainer(container.ID, &docker.HostConfig{})
				if err != nil {
					return nil, err
				}
			}

			c, err := pool.Client.InspectContainer(container.ID)
			if err != nil {
				return nil, err
			}
			resource = &dockertest.Resource{
				Container: c,
			}
		}

		host = "127.0.0.1"
		port = resource.GetPort("3306/tcp")
	}

	db, err := getConnection(pool, host, port, "")
	if err != nil {
		return nil, err
	}

	err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", dbName)).Error
	if err != nil {
		return nil, err
	}
	sql := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`;", dbName)
	err = db.Exec(sql).Error
	if err != nil {
		return nil, err
	}
	db, err = getConnection(pool, host, port, dbName)
	if err != nil {
		return nil, err
	}

	d := &Database{
		Name:     dbName,
		pool:     pool,
		resource: resource,
		host:     host,
		port:     port,
		DB:       db,
		isGithub: isGithub,
	}

	return d, nil
}

func getConnection(pool *dockertest.Pool, host, port, dbName string) (*gorm.DB, error) {
	var db *gorm.DB
	err := pool.Retry(func() error {
		var err error
		mysqlConn := mysql.Open(fmt.Sprintf("root:root@(%s:%s)/%s?charset=utf8&parseTime=true&multiStatements=true&&sql_mode='ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION'", host, port, dbName))
		db, err = gorm.Open(mysqlConn, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			return err
		}
		conn, err := db.DB()
		if err != nil {
			return err
		}
		return conn.Ping()
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// StopDB stop and remove the docker of db for unit test
func (d *Database) StopDB() error {
	// For github environment, let github deal with it
	if d.isGithub {
		return nil
	}

	_, reuse := os.LookupEnv("REUSE_DOCKER")
	if reuse {
		return nil
	}

	err := d.pool.Purge(d.resource)
	if err != nil {
		return err
	}
	return nil
}

// GetDBName get a db name by using the Suite struct in each test
func GetDBName(s interface{}) string {
	return strings.ReplaceAll(reflect.TypeOf(s).Name(), "/", "_")
}

// InitDB init database schema and data
func (d *Database) InitDB() error {
	return nil
}

// ClearDB drop the tables in database
func (d *Database) ClearDB() error {
	// Drop tables
	// #nosec
	sql := fmt.Sprintf("SELECT concat('DROP TABLE IF EXISTS `', table_name, '`;') AS s FROM information_schema.tables WHERE table_schema = '%s';", d.Name)
	dropSQLs := []struct {
		S string `gorm:"column:s"`
	}{}
	err := d.DB.Raw(sql).Scan(&dropSQLs).Error
	if err != nil {
		return err
	}
	for i := range dropSQLs {
		err = d.DB.Exec(dropSQLs[i].S).Error
		if err != nil {
			return err
		}
	}

	// Drop functions
	// #nosec
	sql = fmt.Sprintf("SELECT CONCAT('DROP ',ROUTINE_TYPE,' `',ROUTINE_SCHEMA,'`.`',ROUTINE_NAME,'`;') as s FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA='%s';", d.Name)
	err = d.DB.Raw(sql).Scan(&dropSQLs).Error
	if err != nil {
		return err
	}

	for i := range dropSQLs {
		err = d.DB.Exec(dropSQLs[i].S).Error
		if err != nil {
			return err
		}
	}

	return nil
}

// InitData init data for testing
func (d *Database) InitData() error {
	return nil
}
