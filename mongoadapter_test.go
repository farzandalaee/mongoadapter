package mongoadapter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	//"fmt"
	"github.com/joho/godotenv"
	"github.com/rs/xid"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var mongoDatabase string
var mongoColl string
var mongoHost string
var mongoPort int
var mongo2Host string
var mongo2Port int
var mongoReadTimeout time.Duration
var mongoWriteTimeout time.Duration
var mongoConnectionTimeout time.Duration
var mongoMaxConnIdleTime time.Duration
var mongoMaxPoolSize uint64
var mongoMinPoolSize uint64

var mongoConfig, mongo2Config *MongoConfig

func CountCursor(cur *mongo.Cursor) int {
	var i = 0
	for cur.Next(context.TODO()) {
		i++
	}
	return i
}

func TestMain(m *testing.M) {
	err := godotenv.Load()
	if err != nil {
		log.Panic("Cannot read .env file.")
	}
	mongoDatabase = os.Getenv("GOTEST_MONGO_DB")
	mongoColl = os.Getenv("GOTEST_MONGO_COLL")
	mongoHost = os.Getenv("GOTEST_MONGO_HOST")
	mongoPort, err = strconv.Atoi(os.Getenv("GOTEST_MONGO_PORT"))
	mongo2Host = os.Getenv("GOTEST_MONGO2_HOST")
	mongo2Port, err = strconv.Atoi(os.Getenv("GOTEST_MONGO2_PORT"))
	t, _ := strconv.Atoi(os.Getenv("GOTEST_MONGO_READ_TIMEOUT"))
	mongoReadTimeout = time.Duration(t) * time.Second
	t, _ = strconv.Atoi(os.Getenv("GOTEST_MONGO_READ_TIMEOUT"))
	mongoWriteTimeout = time.Duration(t) * time.Second
	t, _ = strconv.Atoi(os.Getenv("GOTEST_MONGO_CONN_TIMEOUT"))
	mongoConnectionTimeout = time.Duration(t) * time.Second

	t, _ = strconv.Atoi(os.Getenv("GOTEST_MONGO_MAX_CONN_IDLE_TIME"))
	mongoMaxConnIdleTime = time.Duration(t) * time.Second
	t, _ = strconv.Atoi(os.Getenv("GOTEST_MONGO_MAX_POOL_SIZE"))
	mongoMaxPoolSize = uint64(t)
	t, _ = strconv.Atoi(os.Getenv("GOTEST_MONGO_MIN_POOL_SIZE"))
	mongoMinPoolSize = uint64(t)


	mongoConfig = &MongoConfig{
		Host: mongoHost,
		Port: mongoPort,
		ReadTimeout: mongoReadTimeout,
		WriteTimeout: mongoWriteTimeout,
		ConnTimeout: mongoConnectionTimeout,
		MaxConnIdleTime: mongoMaxConnIdleTime,
		MaxPoolSize: mongoMaxPoolSize,
		MinPoolSize: mongoMinPoolSize,
	}

	mongo2Config = &MongoConfig{
		Host: mongo2Host,
		Port: mongo2Port,
		ReadTimeout: mongoReadTimeout,
		WriteTimeout: mongoWriteTimeout,
		ConnTimeout: mongoConnectionTimeout,
		MaxConnIdleTime: mongoMaxConnIdleTime,
		MaxPoolSize: mongoMaxPoolSize,
		MinPoolSize: mongoMinPoolSize,
	}

	mongo := getMongoConnection(mongoConfig)
	err = mongo.conn.Database(mongoDatabase).Collection(mongoColl).Drop(context.Background())
	if err != nil {
		log.Println("Couldn't drop the collection, it might not exists, if it exists, the test might yield bad results.")
	}

	os.Exit(m.Run())
}

func createRandomName() string {
	return xid.New().String()
}

func CreateRandomMobileNumber(prefix string) string {
	var min = 1000000
	var max = 9999999
	rand.Seed(time.Now().UnixNano())
	return prefix + strconv.Itoa(rand.Intn(max-min)+min)
}

type DummyUser struct {
	Name string `bson:"name"`
	Email string `bson:"email"`
}

func insertDummyUser(db, coll string, num int) ([]interface{}, error) {
	var users = make([]interface{}, num)
	var i int
	for i < num {
		var rn = CreateRandomMobileNumber("")
		users[i] = DummyUser{
			Name : "user-"+rn,
			Email : "user-"+rn+"@email.com",
		}
		i++
	}
	m, _ := NewMongo(mongoConfig)
	many, err := m.InsertMany(db, coll, users)
	if err != nil {
		return nil, err
	}
	if len(many.InsertedIDs) == num {
		return users, nil
	}
	return nil, errors.New("Failed to insert some records.")
}

func getMongoConnection(Config *MongoConfig) *Mongo {
	Destroy(Config.Host, Config.Port)
	mongoInstance, err := NewMongo(Config)
	if err != nil {
		log.Panic("Failed to connect to mongoDb")
	}
	return mongoInstance
}

func TestNewMongo_mustAssertTrueOnConnection(t *testing.T) {
	Destroy(mongoConfig.Host, mongoConfig.Port)
	m, err := NewMongo(mongoConfig)
	assert.Nil(t, err)
	assert.IsType(t, Mongo{}, *m)
}

func TestNewMongo_mustAssertErr(t *testing.T) {
	Destroy(mongoConfig.Host, mongoConfig.Port)
	var wrongConfig = *mongoConfig
	wrongConfig.Host = "wrong"
	m, err := NewMongo(&wrongConfig)
	assert.Error(t, err)
	assert.Nil(t, m)
}

func TestNewMongo_mustAssertTrue_zeroTimeouts(t *testing.T) {
	Destroy(mongoConfig.Host, mongoConfig.Port)
	m, err := NewMongo(mongoConfig)
	assert.Nil(t, err)
	assert.IsType(t, Mongo{}, *m)
}

func TestNewMongo_mustAssertTrue_singleInstanceOnly(t *testing.T) {
	m, _ := NewMongo(mongoConfig)
	m2, _ := NewMongo(mongoConfig)
	assert.Equal(t, m.ID, m2.ID)
}


func TestNewMongo_mustAssertTrue_AssertTrueForTwoDifferentInstances(t *testing.T) {
	m, _ := NewMongo(mongoConfig)
	m2, _ := NewMongo(mongo2Config)
	assert.NotEqual(t, m.ID, m2.ID)
}

func TestMongo_InsertOne(t *testing.T) {
	data := bson.M{"name": "tester", "email": "test@test-domain.com"}
	m, _ := NewMongo(mongoConfig)
	r, err := m.InsertOne(mongoDatabase, mongoColl, data)
	assert.Nil(t, err)
	assert.IsType(t, mongo.InsertOneResult{}, *r)
}

func TestMongo_FindOne(t *testing.T) {
	m, _ := NewMongo(mongoConfig)
	r := m.FindOne(mongoDatabase, mongoColl, bson.D{{"name", "tester"}})
	var du DummyUser
	err := r.Decode(&du)
	assert.Nil(t, err)
	assert.Equal(t, du.Name, "tester")
}

func TestMongo_FindOneNoDocumentError(t *testing.T) {
	m, _ := NewMongo(mongoConfig)
	r := m.FindOne(mongoDatabase, mongoColl, bson.D{{"name", "aNotFoundName4567568679789098"}})
	var du DummyUser
	err := r.Decode(&du)
	assert.True(t, m.NoDocument(err))
}

func TestMongo_InsertMany(t *testing.T) {
	m, _ := NewMongo(mongoConfig)
	type Entry struct {
		Name string	`bson:"name"`
		Email string `bson:"email"`
	}
	var entries []interface{}
	entries = append(entries, Entry{ Name: "man-01",  Email: "email1@site.com"})
	entries = append(entries, Entry{ Name: "man-02",  Email: "email2@site.com"})
	entries = append(entries, Entry{ Name: "man-03",  Email: "email3@site.com"})
	r, err := m.InsertMany(mongoDatabase, mongoColl, entries)
	assert.Nil(t, err)
	assert.IsType(t, mongo.InsertManyResult{}, *r)
	assert.Equal(t, 3, len(r.InsertedIDs))
}

func TestMongo_FindWhereIn(t *testing.T) {
	m, _ := NewMongo(mongoConfig)
	r, err := m.FindWhereIn(mongoDatabase, mongoColl, false, []string{"name", "man-02", "man-03"})
	assert.Nil(t, err)
	assert.Equal(t, 2, CountCursor(r))
}

func TestMongo_FindWhereInMulti(t *testing.T) {
	m, _ := NewMongo(mongoConfig)
	many, err := insertDummyUser(mongoDatabase, "dummyUsers", 10)
	if err != nil {
		t.Fatal("Fail to create dummy users for TestMongo_FindWhereInNot")
	}
	var name = many[0].(DummyUser).Name
	var name2 = many[1].(DummyUser).Name
	var email = many[2].(DummyUser).Email
	var email2 = many[3].(DummyUser).Email
	r, err := m.FindWhereIn(mongoDatabase, "dummyUsers", false, []string{"name", name, name2},
		[]string{"email", email, email2})
	assert.Nil(t, err)
	assert.Equal(t, 4, CountCursor(r))
}


func TestMongo_FindWhereInNot(t *testing.T) {
	var totalUsers = 8
	m, _ := NewMongo(mongoConfig)
	many, err := insertDummyUser(mongoDatabase, "dummyUsersWhereNotInOneTime", totalUsers)
	if err != nil {
		t.Fatal("Fail to create dummy users for TestMongo_FindWhereInNot")
	}
	var name string
	if usersField, ok := many[0].(DummyUser); ok {
		name = usersField.Name
	} else {
		t.Fatal("Failed to type assert the DummyUser objex")
	}
	r, err := m.FindWhereIn(mongoDatabase, "dummyUsersWhereNotInOneTime", true, []string{"name", name})
	assert.Nil(t, err)
	assert.Equal(t, totalUsers-1, CountCursor(r))
}

func TestMongo_UpdateOne_MustAssertTrue(t *testing.T) {
	m, _ := NewMongo(mongoConfig)
	r, err := m.UpdateOne(mongoDatabase, mongoColl, bson.D{{ "name", "man-01"}}, bson.D{{"$set", bson.D{{ "email", "changed@test.com"}}}})
	assert.Nil(t, err)
	assert.IsType(t, &mongo.UpdateResult{}, r)
	assert.Equal(t, 1, int(r.MatchedCount))
	rr := m.FindOne(mongoDatabase, mongoColl, bson.D{ { "email", "changed@test.com" }})
	var result DummyUser
	assert.NoError(t, rr.Decode(&result))
	assert.NotNil(t, rr)
}

func TestMongo_UpdateOne_MustAssertFail(t *testing.T) {
	m, _ := NewMongo(mongoConfig)
	r, err := m.UpdateOne(mongoDatabase, mongoColl, bson.D{{ "name", "man-0100"}}, bson.D{{"$set", bson.D{{ "email", "changed@test.com"}}}})
	assert.Nil(t, err)
	assert.IsType(t, &mongo.UpdateResult{}, r)
	assert.Equal(t, 0, int(r.MatchedCount))
}

func TestMongo_DeleteOne_MustAssertTrue(t *testing.T) {
	m, _ := NewMongo(mongoConfig)
	r, err := m.DeleteOne(mongoDatabase, mongoColl, bson.D{{ "name", "man-01"}})
	assert.Nil(t, err)
	assert.IsType(t, &mongo.DeleteResult{}, r)
	assert.Equal(t, 1, int(r.DeletedCount))
}

func TestMongo_DeleteMany_MustAssertTrue(t *testing.T) {
	m, _ := NewMongo(mongoConfig)
	r, err := m.DeleteMany(mongoDatabase, mongoColl, bson.D{{ "name", primitive.Regex{Pattern: "m.*", Options: ""}}})
	assert.Nil(t, err)
	assert.IsType(t, &mongo.DeleteResult{}, r)
	assert.Equal(t, 2, int(r.DeletedCount))
}

func TestMongo_GetID_MustAssertTrue(t *testing.T) {
	data := bson.M{"name": "testingForID", "email": "test@test-domain.com"}
	m, _ := NewMongo(mongoConfig)
	r, err := m.InsertOne(mongoDatabase, mongoColl, data)
	assert.Nil(t, err)
	assert.IsType(t, mongo.InsertOneResult{}, *r)
	id, err := m.GetID(r.InsertedID)
	assert.NoError(t, err)
	assert.IsType(t, "", id)
}

func TestMongo_AddUniqueIndex(t *testing.T) {
	//data := bson.M{"name": "testingForID", "email": "test@test-domain.com"}
	m, _ := NewMongo(mongoConfig)
	_, _ = m.InsertOne("cart", "coupon", bson.M{"code": "kir"})
	s, err := m.AddUniqueIndex("cart", "coupon", "code")
	assert.NoError(t, err)
	assert.IsType(t, "", s)
}


func TestMongo_Count(t *testing.T) {
	m, _ := NewMongo(mongoConfig)
	rand.Seed(time.Now().Unix())
	var randNum = rand.Intn(150)
	var collName = fmt.Sprintf("countCollTest%v", time.Now().Unix())
	for i := 0; i < randNum; i++ {
		res, err := m.InsertOne(mongoDatabase, collName, &DummyUser{Name: fmt.Sprintf("test-name-%v", i),
			Email: fmt.Sprintf("test-email-%v@email.com", i)})
		_ = res
		_ = err
	}
	count, err := m.Count(mongoDatabase, collName, bson.M{})
	assert.NoError(t, err)
	assert.Equal(t, int64(randNum), count)
}

func TestMongo_EstimatedCount(t *testing.T) {
	m, _ := NewMongo(mongoConfig)
	rand.Seed(time.Now().Unix())
	var randNum = rand.Intn(150)
	var collName = fmt.Sprintf("countCollTestEst%v", time.Now().Unix())
	for i := 0; i < randNum; i++ {
		res, err := m.InsertOne(mongoDatabase, collName, &DummyUser{Name: fmt.Sprintf("test-name-%v", i),
			Email: fmt.Sprintf("test-email-%v@email.com", i)})
		_ = res
		_ = err
	}
	count, err := m.EstimatedCount(mongoDatabase, collName)
	assert.NoError(t, err)
	assert.Equal(t, int64(randNum), count)
}

func TestMongo_Search(t *testing.T) {
	m, _ := NewMongo(mongoConfig)
	type SearchProductEntry struct {
		Title string `bson:"title"`
		Stock int `bson:"stock"`
	}
	type SearchEntry struct {
		Name string `bson:"name"`
		Mobile string `bson:"mobile"`
		Email string `bson:"email"`
		Date time.Time `bson:"date"`
		OrderNum int `bson:"orderNum"`
		KeyIndex string `bson:"keyIndex"`
		Products []SearchProductEntry `bson:"products"`
	}
	collName := "entries"
	_ = m.conn.Database("searchEntry").Collection(collName).Drop(context.Background())
	var j = 0
	for i := 0; i < 999; i++ {
		randNum := CreateRandomMobileNumber("")
		orderNum := 5
		if j % 2 == 0 {
			orderNum = 2
		}
		ent := SearchEntry{
			Name: randNum+"searchName",
			Mobile: "0935"+randNum,
			Email: "email"+randNum+"faza-test.io",
			Date: time.Now().UTC(),
			OrderNum:orderNum,
			KeyIndex: strconv.Itoa(j),
			Products: []SearchProductEntry{{Title: "1-prdSearch"+randNum, Stock: i}, {Title: "2-prdSearch"+randNum, Stock: j}},
		}
		_, _ = m.InsertOne("searchEntry", collName, ent)
		j++
	}

	var filters = map[string][]string{"name" : {"searchName", "like"}, "email" : {"faza-test", "like"}}
	cur, err := m.Search("searchEntry", "entries", filters, map[string]int{"name" : -1}, 2, 5)
	assert.NoError(t, err)
	var entries []SearchEntry
	for cur.Next(context.Background()) {
		var tmp SearchEntry
		_ = cur.Decode(&tmp)
		entries = append(entries, tmp)
	}
	assert.True(t, len(entries) > 0)
}
func TestMongo_SearchCount(t *testing.T) {
	m, _ := NewMongo(mongoConfig)
	type SearchProductEntry struct {
		Title string `bson:"title"`
		Stock int `bson:"stock"`
	}
	type SearchEntry struct {
		Name string `bson:"name"`
		Mobile string `bson:"mobile"`
		Email string `bson:"email"`
		Date time.Time `bson:"date"`
		OrderNum int `bson:"orderNum"`
		KeyIndex string `bson:"keyIndex"`
		Products []SearchProductEntry `bson:"products"`
	}
	collName := "entriesForCountTest"
	_ = m.conn.Database("searchEntry").Collection(collName).Drop(context.Background())
	var j = 0
	for i := 0; i < 999; i++ {
		randNum := CreateRandomMobileNumber("")
		orderNum := 5
		if j % 2 == 0 {
			orderNum = 2
		}
		ent := SearchEntry{
			Name: randNum+"searchName",
			Mobile: "0935"+randNum,
			Email: "email"+randNum+"faza-test.io",
			Date: time.Now().UTC(),
			OrderNum:orderNum,
			KeyIndex: strconv.Itoa(j),
			Products: []SearchProductEntry{{Title: "1-prdSearch"+randNum, Stock: i}, {Title: "2-prdSearch"+randNum, Stock: j}},
		}
		_, _ = m.InsertOne("searchEntry", collName, ent)
		j++
	}

	var filters = map[string][]string{"name" : {"searchName", "like"}}
	count, err := m.SearchCount("searchEntry", collName, filters)
	assert.NoError(t, err)
	assert.Equal(t, int64(999), count)
}

