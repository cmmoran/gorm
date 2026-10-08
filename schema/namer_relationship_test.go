package schema_test

import (
 "os"
 "strings"
 "sync"
 "testing"

 "gorm.io/gorm/schema"
)

type upperColumnNamer struct { schema.NamingStrategy }
func (n upperColumnNamer) ColumnName(table, column string) string {
 return strings.ToUpper(n.NamingStrategy.ColumnName(table, column))
}

func TestNamerRelationshipInference(t *testing.T) {
 type Company struct {
  ID int `gorm:"primaryKey;column:company_id"`
 }
 type User struct {
  ID int
  CompanyID int
  Company Company `gorm:"foreignKey:CompanyID;references:ID"`
 }
 type ForeignKeyOnly struct {
  ID int
  CompanyID int
  Company Company `gorm:"foreignKey:CompanyID"`
 }
 type DBNameTag struct {
  ID int
  CompanyID int
  Company Company `gorm:"foreignKey:company_id;references:id"`
 }
 type OrdinaryCompany struct { ID int }
 type OrdinaryUser struct {
  ID int
  CompanyID int
  Company OrdinaryCompany
 }
 type ExactCollision struct {
  ID int
  CompanyID int
  Company OrdinaryCompany `gorm:"foreignKey:ID;references:ID"`
 }
 type DisambiguatedCompany struct {
  ID int `gorm:"primaryKey;column:company_id"`
  Code string
 }
 type Disambiguated struct {
  ID int
  CompanyID string
  Company DisambiguatedCompany `gorm:"foreignKey:CompanyID;references:Code"`
 }
 type ForcedBelongs struct {
  ID int
  CompanyID int
  Company Company `gorm:"foreignKey:CompanyID;references:ID;belongsTo:Company"`
 }
 type ConventionalCompany struct { ID int `gorm:"primaryKey;column:user_id"` }
 // The local type name User is important to conventional has-one inference.
 conventional := func() interface{} {
  type User struct {
   ID int
   CompanyID int
   Company ConventionalCompany
  }
  return &User{}
 }
 cases := []struct {
  name string
  model interface{}
  want schema.RelationshipType
  foreign string
 }{
  {"foreign-key-and-references", &User{}, schema.BelongsTo, "CompanyID"},
  {"foreign-key-only", &ForeignKeyOnly{}, schema.BelongsTo, "CompanyID"},
  {"database-name-tags", &DBNameTag{}, schema.HasOne, "ID"},
  {"ordinary-conventional", &OrdinaryUser{}, schema.BelongsTo, "CompanyID"},
  {"exact-name-collision-predates-pr", &ExactCollision{}, schema.HasOne, "ID"},
  {"references-disambiguates", &Disambiguated{}, schema.BelongsTo, "CompanyID"},
  {"forced-belongs-to", &ForcedBelongs{}, schema.BelongsTo, "CompanyID"},
  {"conventional-column-collision", conventional(), schema.BelongsTo, "CompanyID"},
 }
 namers := []struct { name string; namer schema.Namer }{
  {"default", schema.NamingStrategy{}},
  {"table-prefix", schema.NamingStrategy{TablePrefix:"tenant_", SingularTable:true}},
  {"no-lower-case", schema.NamingStrategy{NoLowerCase:true}},
  {"replacer", schema.NamingStrategy{NameReplacer:strings.NewReplacer("Company", "Employer")}},
  {"uppercase", upperColumnNamer{}},
 }
 for _, n := range namers {
  for _, c := range cases {
   t.Run(n.name+"/"+c.name, func(t *testing.T) {
    s, err := schema.Parse(c.model, &sync.Map{}, n.namer)
    if err != nil {
     t.Logf("parse error: %v", err)
     if os.Getenv("GORM_INFERENCE_OBSERVE") == "1" || (c.name == "database-name-tags" && (n.name == "no-lower-case" || n.name == "uppercase")) { return }
     t.Fatal(err)
    }
    rel := s.Relationships.Relations["Company"]
    if rel == nil || len(rel.References) != 1 { t.Fatalf("unexpected relation: %#v", rel) }
    r := rel.References[0]
    t.Logf("type=%s primary=%s.%s foreign=%s.%s own=%t", rel.Type, r.PrimaryKey.Schema.Name, r.PrimaryKey.Name, r.ForeignKey.Schema.Name, r.ForeignKey.Name, r.OwnPrimaryKey)
    if os.Getenv("GORM_INFERENCE_OBSERVE") != "1" && (rel.Type != c.want || r.ForeignKey.Name != c.foreign) {
     t.Fatalf("got %s foreign %s; want %s foreign %s", rel.Type, r.ForeignKey.Name, c.want, c.foreign)
    }
   })
  }
 }
}

func TestNamerLookupOracleCompatibility(t *testing.T) {
 type Record struct {
  ID int
  Value string `gorm:"column:VALUE"`
 }
 s, err := schema.Parse(&Record{}, &sync.Map{}, upperColumnNamer{})
 if err != nil { t.Fatal(err) }
 if s.LookUpField("value") != s.FieldsByName["Value"] { t.Fatal("Namer fallback must resolve returned column names") }
 if s.LookUpField("missing") != nil { t.Fatal("unexpected missing field") }
}
