package mgo

import (
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"log"
	"reflect"
	"starter/pkg/config"
	"strings"
	"time"
)

type collection struct {
	Database *mgo.Session
	Table    *mgo.Collection
	Session  *mgo.Database
	filter   bson.M
	limit    int
	skip     int
	sort     []string
	fields   bson.M
}

var db *mgo.Session

func Start() {
	conf := config.Config.Mongo
	dialInfo := &mgo.DialInfo{
		Addrs:     []string{strings.ReplaceAll(conf.Url, "mongodb://", "")},
		Direct:    false,
		Timeout:   time.Second * 5,
		PoolLimit: conf.MaxPoolSize,
		Username:  conf.Username,
		Password:  conf.Password,
	}
	var err error
	db, err = mgo.DialWithInfo(dialInfo)
	if err != nil {
		log.Fatalln(err)
	}
	db.SetMode(mgo.Monotonic, true)
}

// 得到一个mongo操作对象
// 请显示调用 Close 方法释放session
func Collection(table string) *collection {
	database := db.Copy()
	session := database.DB(config.Config.Mongo.Database)
	return &collection{
		Database: database,
		Session:  session,
		Table:    session.C(table),
		filter:   make(bson.M),
	}

}

func (collection *collection) Where(m bson.M) *collection {
	collection.filter = m
	return collection
}

func (collection *collection) Close() {
	collection.Database.Close()
}

// 限制条数
func (collection *collection) Limit(n int) *collection {
	collection.limit = n
	return collection
}

// 跳过条数
func (collection *collection) Skip(n int) *collection {
	collection.skip = n
	return collection
}

// 排序 bson.M{"created_at":-1}
func (collection *collection) Sort(sorts ...string) *collection {
	collection.sort = sorts
	return collection
}

// 指定查询字段
func (collection *collection) Fields(fields bson.M) *collection {
	collection.fields = fields
	return collection
}

// 写入单条数据
func (collection *collection) InsertOne(document interface{}) (interface{}, error) {
	data := BeforeCreate(document)
	err := collection.Table.Insert(data)
	if err != nil {
		log.Println(err)
	}
	return data, err
}

// 写入多条数据
func (collection *collection) InsertMany(documents interface{}) interface{} {
	var data []interface{}
	data = BeforeCreate(documents).([]interface{})
	err := collection.Table.Insert(data)
	if err != nil {
		log.Println(err)
	}
	return data
}

// 存在更新,不存在写入, documents 里边的文档需要有 _id 的存在
func (collection *collection) UpdateOrInsert(document interface{}) *mgo.ChangeInfo {
	result, err := collection.Table.Upsert(collection.filter, document)
	if err != nil {
		log.Println(err)
	}
	return result
}

//
func (collection *collection) UpdateOne(document interface{}) bool {
	err := collection.Table.Update(collection.filter, bson.M{"$set": BeforeUpdate(document)})
	if err != nil {
		log.Println(err)
	}
	return err == nil
}

//
func (collection *collection) UpdateMany(document interface{}) *mgo.ChangeInfo {
	result, err := collection.Table.UpdateAll(collection.filter, bson.M{"$set": BeforeUpdate(document)})
	if err != nil {
		log.Println(err)
	}
	return result
}

// 查询一条数据
func (collection *collection) FindOne(document interface{}) error {
	err := collection.Table.Find(collection.filter).Select(collection.fields).One(document)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

// 查询多条数据
func (collection *collection) FindMany(documents interface{}) {
	err := collection.Table.Find(collection.filter).Skip(collection.skip).Limit(collection.limit).Sort(collection.sort...).Select(collection.fields).All(documents)
	if err != nil {
		log.Println(err)
	}
}

// 删除数据,并返回删除成功的数量
func (collection *collection) Delete() bool {
	if collection.filter == nil || len(collection.filter) == 0 {
		log.Println("you can't delete all documents, it's very dangerous")
		return false
	}
	err := collection.Table.Remove(collection.filter)
	if err != nil {
		log.Println(err)
	}
	return err == nil
}

func (collection *collection) Count() int64 {
	count, err := collection.Table.Find(collection.filter).Count()
	if err != nil {
		log.Println(err)
		return 0
	}
	return int64(count)
}

func BeforeCreate(document interface{}) interface{} {
	val := reflect.ValueOf(document)
	typ := reflect.TypeOf(document)

	switch typ.Kind() {
	case reflect.Ptr:
		return BeforeCreate(val.Elem().Interface())

	case reflect.Array, reflect.Slice:
		var sliceData = make([]interface{}, val.Len(), val.Cap())
		for i := 0; i < val.Len(); i++ {
			sliceData[i] = BeforeCreate(val.Index(i).Interface()).(bson.M)
		}
		return sliceData

	case reflect.Struct:
		var data = make(bson.M)
		for i := 0; i < typ.NumField(); i++ {
			data[typ.Field(i).Tag.Get("bson")] = val.Field(i).Interface()
		}
		dataVal := reflect.ValueOf(data)
		if val.FieldByName("Id").Type() == reflect.TypeOf(bson.ObjectId("")) {
			dataVal.SetMapIndex(reflect.ValueOf("_id"), reflect.ValueOf(bson.NewObjectId()))
		}

		if val.FieldByName("Id").Interface() == "" {
			dataVal.SetMapIndex(reflect.ValueOf("_id"), reflect.ValueOf(bson.NewObjectId().String()))
		}

		dataVal.SetMapIndex(reflect.ValueOf("created_at"), reflect.ValueOf(time.Now().Unix()))
		dataVal.SetMapIndex(reflect.ValueOf("updated_at"), reflect.ValueOf(time.Now().Unix()))
		return dataVal.Interface()

	default:
		if val.Type() == reflect.TypeOf(bson.M{}) {
			if !val.MapIndex(reflect.ValueOf("_id")).IsValid() {
				val.SetMapIndex(reflect.ValueOf("_id"), reflect.ValueOf(bson.NewObjectId()))
			}
			val.SetMapIndex(reflect.ValueOf("created_at"), reflect.ValueOf(time.Now().Unix()))
			val.SetMapIndex(reflect.ValueOf("updated_at"), reflect.ValueOf(time.Now().Unix()))
		}
		return val.Interface()
	}
}

func BeforeUpdate(document interface{}) interface{} {
	val := reflect.ValueOf(document)
	typ := reflect.TypeOf(document)

	switch typ.Kind() {
	case reflect.Ptr:
		return BeforeUpdate(val.Elem().Interface())

	case reflect.Array, reflect.Slice:
		var sliceData = make([]interface{}, val.Len(), val.Cap())
		for i := 0; i < val.Len(); i++ {
			sliceData[i] = BeforeCreate(val.Index(i).Interface()).(bson.M)
		}
		return sliceData

	case reflect.Struct:
		var data = make(bson.M)
		for i := 0; i < typ.NumField(); i++ {
			data[typ.Field(i).Tag.Get("bson")] = val.Field(i).Interface()
		}
		dataVal := reflect.ValueOf(data)
		dataVal.SetMapIndex(reflect.ValueOf("updated_at"), reflect.ValueOf(time.Now().Unix()))
		return dataVal.Interface()

	default:
		if val.Type() == reflect.TypeOf(bson.M{}) {
			val.SetMapIndex(reflect.ValueOf("updated_at"), reflect.ValueOf(time.Now().Unix()))
		}
		return val.Interface()
	}
}