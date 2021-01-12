package mongoadapter

import (
	"context"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/x/bsonx"

	"strconv"
	"time"

	"github.com/matryer/resync"
	"github.com/rs/xid"
	//"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoConfig struct {
	Host string
	Port int
	Username string
	Password string
	ConnTimeout 	time.Duration
	ReadTimeout 	time.Duration
	WriteTimeout 	time.Duration
	MaxConnIdleTime	time.Duration
	MaxPoolSize		uint64
	MinPoolSize		uint64
}

type Mongo struct {
	ID string
	conn         *mongo.Client
	readTimeout  time.Duration
	writeTimeout time.Duration
}

type TotalCount struct {
	TotalCount int64 `bson:"totalCount"`
}

// using sync package to ensure our instantiation
// of mongo under high concurrency does happen
// only and only once
var mongoOnceCollection map[string]*resync.Once
var mongoInstanceCollection map[string]*Mongo


// NewMongo returns an instance of Mongo with an established connection. It uses the Ping()
// method to ensure the healthiness of the connection, in case Ping() returns error, the method
// aborts and returns an error accordingly.
// If there is already a Mongo instance created for the given host+":"+port combination,
// it returns it. It does NOT create a new connection+instance for host and port combination
// if it has already done so.
func NewMongo(Config *MongoConfig) (*Mongo, error) {
	var auth = string("")
	if Config.Port == 0 {
		return nil, errors.New("invalid port, port must be a non-zero integer")
	}
	if Config.Username != "" && Config.Password != "" {
		auth = fmt.Sprintf("%v:%v@", Config.Username, Config.Password)
	}
	var uri = Config.Host+":"+strconv.Itoa(Config.Port)
	if mongoOnceCollection == nil {
		mongoOnceCollection = make(map[string]*resync.Once)
	}
	if mongoInstanceCollection == nil {
		mongoInstanceCollection = make(map[string]*Mongo)
	}
	if _, ok := mongoOnceCollection[uri]; !ok {
		mongoOnceCollection[uri] = &resync.Once{}
	}

	// we do a check to see if our global app
	// container already has an instance of Mongo
	// or not
	if _, ok := mongoInstanceCollection[uri]; ok {
		return mongoInstanceCollection[uri], nil
	}

	var mainErr error

	mongoOnceCollection[uri].Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), Config.ConnTimeout*time.Second)
		defer cancel()
		var uid = xid.New()
		var mongoUri = fmt.Sprintf("mongodb://%v%v:%v", auth, Config.Host, Config.Port)
		clientOptions := options.Client().ApplyURI(mongoUri)

		if Config.MaxConnIdleTime != 0 {
			maxConnIdleTime := Config.MaxConnIdleTime * time.Second
			clientOptions.MaxConnIdleTime = &maxConnIdleTime
		}

		if Config.MaxPoolSize != 0 {
			clientOptions.MaxPoolSize = &Config.MaxPoolSize
		}

		if Config.MinPoolSize != 0 {
			clientOptions.MinPoolSize = &Config.MinPoolSize
		}

		client, err := mongo.Connect(ctx, clientOptions)

		if err != nil {
			mainErr = errors.New("failed to Connect() to mongo, got error: "+ err.Error())
			return
		}

		// Check the connection
		var ctx2, _ = context.WithTimeout(context.Background(), Config.ConnTimeout*time.Second)
		err = client.Ping(ctx2, nil)

		if err != nil {
			mainErr = errors.New("testing MongoDB connection with Ping() method failed, got error: "+ err.Error())
			return
		}

		if Config.ReadTimeout == 0 {
			Config.ReadTimeout = 5
		}

		if Config.WriteTimeout == 0 {
			Config.WriteTimeout = 5
		}

		mongoInstanceCollection[uri] = &Mongo {
			ID:			uid.String(),
			readTimeout:  time.Duration(Config.ReadTimeout),
			writeTimeout: time.Duration(Config.WriteTimeout),
			conn:         client,
		}

		return
	})

	if mainErr != nil {
		return nil, mainErr
	}
	return mongoInstanceCollection[uri], nil
}

// Destroy() closes the connection to Mongo (of current mongoInstance) for the
// given host and port combination.
// It also removes the created instance completely, and resets resync.Once object.
// Be careful when using it
func Destroy(host string, port int) {
	var uri = host+":"+strconv.Itoa(port)
	if _, ok := mongoInstanceCollection[uri]; ok {
		_ = mongoInstanceCollection[uri].conn.Disconnect(context.Background())
		delete(mongoInstanceCollection, uri)
	}
	if _, ok := mongoOnceCollection[uri]; ok {
		mongoOnceCollection[uri].Reset()
	}
}

// Returns the connection for current instance,
// used for extending the adapter with more custom functions
func (m *Mongo) GetConn() *mongo.Client {
	return m.conn
}

func (m *Mongo) NoDocument(err error) bool {
	return err == mongo.ErrNoDocuments
}

func (m *Mongo) FindOne(db, coll string, filter interface{}, options ...*options.FindOneOptions) *mongo.SingleResult {
	ctx, _ := context.WithTimeout(context.Background(), m.readTimeout*time.Second)
	return m.conn.Database(db).Collection(coll).FindOne(ctx, filter, options...)
}

func (m *Mongo) FindMany(db, coll string, filter interface{}, options ...*options.FindOptions) (*mongo.Cursor, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.readTimeout*time.Second)
	defer cancel()
	return m.conn.Database(db).Collection(coll).Find(ctx, filter, options...)
}

// FindWhereIn given a set of column and values, it search using $in or $nin operator.
// the format for vars is that each variable is an slice of variable strings; for each variable,
// the first one is the name of field and the rest are the values to be used for searching using $in or $nin based
// in the parameter negate. If negate is true, then $nin is used, else, $in is used.
// so to find all persons named either "robert" or "sara" or john, simply pass two variables
// like this: FindWhereIn(db, coll, []string{"name", "sara", "robert", "john"}).
func (m *Mongo) FindWhereIn(db, coll string, negate bool, vars ...[]string) (*mongo.Cursor, error) {
	if len(vars) == 0 {
		return nil, errors.New("no filter specified for findWhereIn() method. Method execution aborted")
	}
	var operator = "$in"
	if negate == true {
		operator = "$nin"
	}
	var subConditions bson.A
	for _, v := range vars {
		subConditions = append(subConditions,  bson.D{{v[0], bson.D{{operator, v[1:]}}}})
	}
	var conditions = bson.D{{
		"$or", subConditions,
	}}
	ctx, cancel := context.WithTimeout(context.Background(), m.readTimeout*time.Second)
	defer cancel()
	return m.conn.Database(db).Collection(coll).Find(ctx, conditions)
}

// Inserts one record into the given collection of given db
func (m *Mongo) InsertOne(db, coll string, doc interface{}) (*mongo.InsertOneResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.writeTimeout*time.Second)
	defer cancel()
	return m.conn.Database(db).Collection(coll).InsertOne(ctx, doc)
}

// Inserts an array of record into the given collection of given db
func (m *Mongo) InsertMany(db, coll string, docs []interface{}, options ...*options.InsertManyOptions) (*mongo.InsertManyResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.writeTimeout*time.Second)
	defer cancel()
	return m.conn.Database(db).Collection(coll).InsertMany(ctx, docs, options...)
}

func (m *Mongo) UpdateOne(db, coll string, filter interface{}, data interface{}, options... *options.UpdateOptions) (*mongo.UpdateResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.writeTimeout*time.Second)
	defer cancel()
	return m.conn.Database(db).Collection(coll).UpdateOne(ctx, filter, data, options...)
}

func (m *Mongo) UpdateMany(db, coll string, filter interface{}, data interface{}, options... *options.UpdateOptions) (*mongo.UpdateResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.writeTimeout*time.Second)
	defer cancel()
	return m.conn.Database(db).Collection(coll).UpdateMany(ctx, filter, data, options...)
}

func (m *Mongo) DeleteOne(db, coll string, filter interface{}, options... *options.DeleteOptions) (*mongo.DeleteResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.writeTimeout*time.Second)
	defer cancel()
	return m.conn.Database(db).Collection(coll).DeleteOne(ctx, filter, options...)
}

func (m *Mongo) DeleteMany(db, coll string, filter interface{}, options... *options.DeleteOptions) (*mongo.DeleteResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.writeTimeout*time.Second)
	defer cancel()
	return m.conn.Database(db).Collection(coll).DeleteMany(ctx, filter, options...)
}

// returns the string of a mongoDb's ObjectID
// this is to avoid type conversion for each time
// we need to get the ID in string
func (m *Mongo) GetID(id interface{}) (string, error) {
	if oid, ok := id.(primitive.ObjectID); ok {
		return oid.Hex(), nil
	}
	return "", errors.New("failed to get the objectID from the passed value")
}

func (m *Mongo) ToSliceString(data bson.A) []string {
	var result = make([]string, 0)
	for _, v := range data {
		result = append(result, v.(string))
	}
	return result
}

func (m *Mongo) AddUniqueIndex(db, coll , indexKey string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.writeTimeout*time.Second)
	defer cancel()
	indexModel := mongo.IndexModel{
		Keys: bsonx.Doc{{indexKey, bsonx.Int32(1)}},
		Options: options.Index().SetUnique(true),
	}
	return m.conn.Database(db).Collection(coll).Indexes().CreateOne(ctx, indexModel)
}

func (m *Mongo) AddTextV3Index(db, coll , indexKey string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.writeTimeout*time.Second)
	defer cancel()
	indexModel := mongo.IndexModel{
		Keys: bsonx.Doc{{indexKey, bsonx.Int32(1)}},
		Options: options.Index().SetTextVersion(3),
	}
	return m.conn.Database(db).Collection(coll).Indexes().CreateOne(ctx, indexModel)
}

func (m *Mongo) Count(db, coll string, filters interface{}, opts ...*options.CountOptions) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.readTimeout*time.Second)
	defer cancel()

	return m.conn.Database(db).Collection(coll).CountDocuments(ctx, filters, opts...)
}
func (m *Mongo) EstimatedCount(db, coll string, opts ...*options.EstimatedDocumentCountOptions) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.readTimeout*time.Second)
	defer cancel()

	return m.conn.Database(db).Collection(coll).EstimatedDocumentCount(ctx, opts...)
}

// Search does an aggregation query on mongo db. It supports searching with $match and sorting with $sort
// top-level operators. User of this function must specify how search should happen for each individual passed
// filter in the filters param. Currently "eq" and "like" operators are supported. So, to search all
// country fields named italy, you should pass: map[string][]string{"country" : {"italy", "eq"}}
// You can also pass several fields. Currenly, you cannot use $or, $in etc. and other operators.
func (m *Mongo) Search(db, coll string, filters map[string][]string, sorting map[string]int, limit, skip int64) (*mongo.Cursor, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.readTimeout*time.Second)
	defer cancel()
	var rules []bson.M

	// $match operator must come as the first stage in the pipelines
	// this also has performance benefits, such as utilizing the index
	// like FindOne() and FindMany()
	var filteringRule = make(bson.M, len(filters))
	var filteringStmt bson.M
	if filters != nil && len(filters) > 0 {
		for k, v := range filters {
			if v[1] == "like" {
				filteringRule[k] = bson.M{"$regex" : v[0]}
			} else if v[1] == "eq" {
				filteringRule[k] = v[0]
			}
		}
		filteringStmt = bson.M{"$match" : filteringRule}
		rules = append(rules, filteringStmt)
	}

	var sortingStmt bson.M
	var sortRule = make(bson.M, len(sorting))
	if sorting != nil && len(sorting) > 0 {
		for k, v := range sorting {
			sortRule[k] = v
		}
		sortingStmt = bson.M{"$sort": sortRule}
		rules = append(rules, sortingStmt)
	}

	rules = append(rules, bson.M{"$skip" : skip})
	// it is better to put the limit after a possible
	// $sort stage in the pipeline. Mongo uses the limit
	// for sorting, no matter even it comes after it in the
	// pipeline
	var virtualLimit = int64(5000)
	if limit == 0 {
		limit = virtualLimit
	}
	rules = append(rules, bson.M{"$limit" : limit})



	return m.conn.Database(db).Collection(coll).Aggregate(ctx, rules)
}

// it is the same as Search(), but only returns the total count of search
func (m *Mongo) SearchCount(db, coll string, filters map[string][]string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.readTimeout*time.Second)
	defer cancel()
	var rules []bson.M

	// $match operator must come as the first stage in the pipeline
	// this also has performance benefits, such as utilizing the index
	// like FindOne() and FindMany()
	var filteringRule = make(bson.M, len(filters))
	var filteringStmt bson.M
	if filters != nil && len(filters) > 0 {
		for k, v := range filters {
			if v[1] == "like" {
				filteringRule[k] = bson.M{"$regex" : v[0]}
			} else if v[1] == "eq" {
				filteringRule[k] = v[0]
			}
		}
		filteringStmt = bson.M{"$match" : filteringRule}
		rules = append(rules, filteringStmt)
	}


	rules = append(rules, bson.M{"$count" : "totalCount"})


	res, err := m.conn.Database(db).Collection(coll).Aggregate(ctx, rules)
	if err != nil {
		return 0, err
	}
	var cnt TotalCount
	for res.Next(ctx) {
		err = res.Decode(&cnt)
		if m.NoDocument(err) {
			return 0, nil
		} else if err != nil {
			return 0, err
		}
	}
	return cnt.TotalCount, nil
}

func (m *Mongo) Aggregate(db, coll string, pipeline interface{}, options ...*options.AggregateOptions) (*mongo.Cursor, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.readTimeout*time.Second)
	defer cancel()
	return m.conn.Database(db).Collection(coll).Aggregate(ctx, pipeline, options...)
}

// checks to see if an error is duplicate error or not
func (m *Mongo) IsDupError(err error) bool {
	val, ok := err.(mongo.WriteException)
	if ok {
		if val.WriteErrors != nil && len(val.WriteErrors) > 0 {
			for _, val := range val.WriteErrors {
				if val.Code == 11000 {
					return true
				}
			}
		}
	}
	return false
}