package gostore

import (
	"fmt"
	r "github.com/dancannon/gorethink"
	"github.com/dustin/gojson"
	"github.com/mgutz/logxi/v1"
	"strings"
)

var logger = log.New("gostore.rethink")

func NewRethinkObjectStore(session *r.Session, database string) RethinkStore {
	s := RethinkStore{session, database}
	s.CreateDatabase()
	return s
}

type RethinkStore struct {
	Session  *r.Session
	Database string
}

type RethinkRows struct {
	cursor *r.Cursor
}

func (s RethinkRows) Next(dst interface{}) (bool, error) {
	if !s.cursor.Next(dst) {
		//		logger.Debug("Error getting next", "err", s.cursor.Err(), "isNil", s.cursor.IsNil())
		return false, s.cursor.Err()
	}
	return true, nil
}

func (s RethinkRows) Close() {
	s.cursor.Close()
}

func (s RethinkStore) CreateDatabase() (err error) {
	return r.DBCreate(s.Database).Exec(s.Session)
}

func (s RethinkStore) GetStore() interface{} {
	return s.Session
}

func (s RethinkStore) CreateTable(store string, sample interface{}) (err error) {
	_, err = r.DB(s.Database).TableCreate(store).RunWrite(s.Session)

	return
}

func (s RethinkStore) All(count int, skip int, store string) (rrows ObjectRows, err error) {
	result, err := r.DB(s.Database).Table(store).OrderBy(r.OrderByOpts{Index: r.Desc("id")}).Run(s.Session)
	if err != nil {
		return
	}
	rrows = RethinkRows{result}
	return
}

func (s RethinkStore) AllCursor(store string) (ObjectRows, error) {
	result, err := r.DB(s.Database).Table(store).Run(s.Session)
	if err != nil {
		return nil, err
	}
	defer result.Close()
	return RethinkRows{result}, nil
}

//Before will retrieve all old rows that were created before the row with id was created
// [1, 2, 3, 4], before 2 will return [3, 4]
//r.db('worksmart').table('store').orderBy({'index': r.desc('id')}).filter(r.row('schemas')
// .eq('osiloke_tsaboin_silverbird').and(r.row('id').lt('55b54e93f112a16514000057')))
// .pluck('schemas', 'id','tid', 'timestamp', 'created_at').limit(100)
func (s RethinkStore) Before(id string, count int, skip int, store string) (rows ObjectRows, err error) {
	result, err := r.DB(s.Database).Table(store).Filter(r.Row.Field("id").Lt(id)).Limit(count).Skip(skip).Run(s.Session)
	if err != nil {
		return
	}
	defer result.Close()
	//	result.All(dst)
	rows = RethinkRows{result}
	return
}

func (s RethinkStore) FilterBefore(id string, filter map[string]interface{}, count int, skip int, store string) (rows ObjectRows, err error) {
	result, err := r.DB(s.Database).Table(store).Between(
		r.MinVal, id, r.BetweenOpts{RightBound: "closed"}).OrderBy(
		r.OrderByOpts{Index: r.Desc("id")}).Filter(
		filter).Limit(count).Run(s.Session)
	if err != nil {
		return
	}
	//	var dst interface{}
	f, _ := json.Marshal(filter)
	logger.Debug("FilterBefore", "query",
		fmt.Sprintf("r.db('%s').table('%s').between(r.minval, '%s').orderBy({index:r.desc('id')}).filter(%s).limit(%d)",
			s.Database, store, id, string(f), count))
	rows = RethinkRows{result}
	return
}

func (s RethinkStore) FilterBeforeCount(id string, filter map[string]interface{}, count int, skip int, store string) (int64, error) {
	result, err := r.DB(s.Database).Table(store).Between(
		r.MinVal, id).OrderBy(
		r.OrderByOpts{Index: r.Desc("id")}).Filter(
		filter).Count().Run(s.Session)
	defer result.Close()

	var cnt int64
	if err = result.One(&cnt); err != nil {
		return 0, ErrNotFound
	}
	return cnt, nil
}

func (s RethinkStore) FilterSince(id string, filter map[string]interface{}, count int, skip int, store string) (rows ObjectRows, err error) {
	result, err := r.DB(s.Database).Table(store).Between(
		id, r.MaxVal, r.BetweenOpts{LeftBound: "open", Index: "id"}).OrderBy(
		r.OrderByOpts{Index: r.Desc("id")}).Filter(
		filter).Limit(count).Run(s.Session)
	if err != nil {
		return
	}
	//	result.All(dst)
	rows = RethinkRows{result}
	return
}

//This will retrieve all new rows that were created since the row with id was created
// [1, 2, 3, 4], since 2 will return [1]
func (s RethinkStore) Since(id string, count, skip int, store string) (rrows ObjectRows, err error) {
	result, err := r.DB(s.Database).Table(store).Filter(r.Row.Field("id").Gt(id)).Limit(count).Skip(skip).Run(s.Session)
	if err != nil {
		return
	}
	//	result.All(dst)
	rrows = RethinkRows{result}
	return
}

func (s RethinkStore) Get(id, store string, dst interface{}) (err error) {
	result, err := r.DB(s.Database).Table(store).Get(id).Run(s.Session)
	if err != nil {
		//		logger.Error("Get", "err", err)
		return
	}
	defer result.Close()
	if result.Err() != nil {
		return result.Err()
	}
	if err = result.One(dst); err == r.ErrEmptyResult {
		//		logger.Error("Get", "err", err)
		return ErrNotFound
	}
	return nil
}

func (s RethinkStore) Save(store string, src interface{}) (key string, err error) {
	result, err := r.DB(s.Database).Table(store).Insert(src, r.InsertOpts{Durability: "soft"}).RunWrite(s.Session)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate primary key") {
			err = ErrDuplicatePk
		}
		return
	}
	if len(result.GeneratedKeys) > 0 {
		key = result.GeneratedKeys[0]
	}
	return

}

func (s RethinkStore) Update(id string, store string, src interface{}) (err error) {
	_, err = r.DB(s.Database).Table(store).Get(id).Update(src, r.UpdateOpts{Durability: "soft"}).RunWrite(s.Session)
	return

}

func (s RethinkStore) Delete(id string, store string) (err error) {
	_, err = r.DB(s.Database).Table(store).Get(id).Delete(r.DeleteOpts{Durability: "hard"}).RunWrite(s.Session)
	return
}

func (s RethinkStore) Stats(store string) (map[string]interface{}, error) {
	result, err := r.DB(s.Database).Table(store).Count().Run(s.Session)
	if err != nil {
		return nil, err
	}
	defer result.Close()
	var cnt int64
	if err = result.One(&cnt); err != nil {
		return nil, ErrNotFound
	}
	return map[string]interface{}{"count": cnt}, nil
}

func (s RethinkStore) GetByField(name, val, store string, dst interface{}) (err error) {
	result, err := r.DB(s.Database).Table(store).Filter(r.Row.Field(name).Eq(val)).Run(s.Session)
	if err != nil {
		return
	}
	defer result.Close()
	if err = result.One(dst); err == r.ErrEmptyResult {
		return ErrNotFound
	}
	return
}

func (s RethinkStore) FilterGet(filter map[string]interface{}, store string, dst interface{}) (err error) {
	result, err := r.DB(s.Database).Table(store).Filter(filter).Limit(1).Run(s.Session)
	if err != nil {
		return
	}
	defer result.Close()
	if err = result.One(dst); err == r.ErrEmptyResult {
		return ErrNotFound
	}
	return
}

func (s RethinkStore) FilterGetAll(filter map[string]interface{}, count int, skip int, store string) (rrows ObjectRows, err error) {
	//	logger.Debug("Filter get all", "store", store, "filter", filter)
	result, err := r.DB(s.Database).Table(store).OrderBy(
		r.OrderByOpts{Index: r.Desc("id")}).Filter(filter).Limit(count).Skip(skip).Run(s.Session)
	if err != nil {
		return
	}
	rrows = RethinkRows{result}
	return
}

func (s RethinkStore) FilterDelete(filter map[string]interface{}, store string) (err error) {
	_, err = r.DB(s.Database).Table(store).Filter(filter).Delete(r.DeleteOpts{Durability: "soft"}).RunWrite(s.Session)
	if err == r.ErrEmptyResult {
		return ErrNotFound
	}
	return
}

func (s RethinkStore) FilterCount(filter map[string]interface{}, store string) (int64, error) {
	result, err := r.DB(s.Database).Table(store).Filter(filter).Count().Run(s.Session)
	if err != nil {
		return 0, err
	}
	defer result.Close()
	var cnt int64
	if err = result.One(&cnt); err != nil {
		return 0, ErrNotFound
	}
	return cnt, nil
}

func (s RethinkStore) GetByFieldsByField(name, val, store string, fields []string, dst interface{}) (err error) {
	result, err := r.DB(s.Database).Table(store).Filter(r.Row.Field(name).Eq(val)).Pluck(fields).Run(s.Session)
	if err != nil {
		return
	}
	defer result.Close()
	if err = result.One(dst); err == r.ErrEmptyResult {
		return ErrNotFound
	}
	return
}

func (s RethinkStore) Close() {
	s.Session.Close()
}
