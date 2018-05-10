package restapi

import (
  "github.com/hashicorp/terraform/helper/schema"
  "fmt"
  "strings"
  "errors"
  "log"
)

func resourceRestApi() *schema.Resource {
  return &schema.Resource{
    Create: resourceRestApiCreate,
    Read:   resourceRestApiRead,
    Update: resourceRestApiUpdate,
    Delete: resourceRestApiDelete,
    Exists: resourceRestApiExists,

    Importer: &schema.ResourceImporter{
      State: resourceRestApiImport,
    },


    Schema: map[string]*schema.Schema{
      "path": &schema.Schema{
        Type:        schema.TypeString,
        Description: "The API path on top of the base URL set in the provider that represents objects of this type on the API server.",
        Required:    true,
      },
      "data": &schema.Schema{
        Type:        schema.TypeString,
        Description: "Valid JSON data that this provider will manage with the API server.",
        Required:    true,
      },
      "debug": &schema.Schema{
        Type:        schema.TypeBool,
        Description: "Whether to emit verbose debug output while working with the API object on the server.",
        Optional:    true,
      },
      "ext": &schema.Schema{
        Type:        schema.TypeString,
        Description: "URL extension",
        Optional:    true,
      },
      "api_data": &schema.Schema{
        Type:        schema.TypeMap,
	Elem:        &schema.Schema{ Type: schema.TypeString },
        Description: "After data from the API server is read, this map will include k/v pairs usable in other terraform resources as readable objects. Currently the value is the golang fmt package's representation of the value (simple primitives are set as expected, but complex types like arrays and maps contain golang formatting).",
	Computed:    true,
      },
    }, /* End schema */

  }
}

/* Simple helper routine to build an api_object struct
   for the various calls terraform will use. Unfortunately,
   terraform cannot just reuse objects, so each CRUD operation
   results in a new object created */
func make_api_object(d *schema.ResourceData, m interface{}) (*api_object, error) {
  log.Printf("resource_api_object.go: make_api_object routine called for id '%s'\n", d.Id())
  obj, err := NewAPIObject (
    m.(*api_client),
    d.Get("path").(string),
    d.Id(),
    d.Get("data").(string),
    d.Get("debug").(bool),
    d.Get("ext").(string),
  )
  return obj, err
}

/* After any operation that returns API data, we'll stuff
   all the k,v pairs into the api_data map so users can
   consume the values elsewhere if they'd like */
func set_resource_state(obj *api_object, d *schema.ResourceData) {
  api_data := make(map[string]string)
  for k, v := range obj.api_data {
    api_data[k] = fmt.Sprintf("%v", v)
  }
  d.Set("api_data", api_data)
}


/* Since there is nothing in the ResourceData structure other
   than the "id" passed on the command line, we have to use an opinionated
   view of the API paths to figure out how to read that object
   from the API */
func resourceRestApiImport(d *schema.ResourceData, meta interface{}) (imported []*schema.ResourceData, err error) {
  input := d.Id()
  n := strings.LastIndex(input, "/")
  if n == -1 { return imported, errors.New("Invalid path to import api_object. Must be /<full path from server root>/<object id><ext>") }

  path := input[0:n]
  d.Set("path", path)

  id := input[n+1:len(input)]
  d.Set("data", fmt.Sprintf(`{ "id": "%s" }`, id))
  d.SetId(id)

  /* Troubleshooting is hard enough. Emit log messages so TF_LOG
     has useful information in case an import isn't working */
  d.Set("debug", true)

  obj, err := make_api_object(d, meta)
  if err != nil { return imported, err }
  log.Printf("resource_api_object.go: Import routine called. Object built:\n%s\n", obj.toString())

  err = obj.read_object()
  if err == nil {
    set_resource_state(obj, d)
    /* Data that we set in the state above must be passed along
       as an item in the stack of imported data */
    imported = append(imported, d)
  }

  return imported, err
}

func resourceRestApiCreate(d *schema.ResourceData, meta interface{}) error {
  obj, err := make_api_object(d, meta)
  if err != nil { return err }
  log.Printf("resource_api_object.go: Create routine called. Object built:\n%s\n", obj.toString())

  err = obj.create_object()
  if err == nil {
    /* Setting terraform ID tells terraform the object was created or it exists */
    d.SetId(obj.id)
    set_resource_state(obj, d)
  }
  return err
}

func resourceRestApiRead(d *schema.ResourceData, meta interface{}) error {
  obj, err := make_api_object(d, meta)
  if err != nil { return err }
  log.Printf("resource_api_object.go: Read routine called. Object built:\n%s\n", obj.toString())

  err = obj.read_object()
  if err == nil {
    /* Setting terraform ID tells terraform the object was created or it exists */
    log.Printf("resource_api_object.go: Read resource. Returned id is '%s'\n", obj.id);
    d.SetId(obj.id)
    set_resource_state(obj, d)
  }
  return err
}

func resourceRestApiUpdate(d *schema.ResourceData, meta interface{}) error {
  obj, err := make_api_object(d, meta)
  if err != nil { return err }

  /* If copy_keys is not empty, we have to grab the latest 
     data so we can copy anything needed before the update */
  client := meta.(*api_client)
  if len(client.copy_keys) > 0 {
    err = obj.read_object()
    if err != nil { return err }
  }

  log.Printf("resource_api_object.go: Update routine called. Object built:\n%s\n", obj.toString())

  err = obj.update_object()
  if err == nil {
    set_resource_state(obj, d)
  }
  return err
}

func resourceRestApiDelete(d *schema.ResourceData, meta interface{}) error {
  obj, err := make_api_object(d, meta)
  if err != nil { return err }
  log.Printf("resource_api_object.go: Delete routine called. Object built:\n%s\n", obj.toString())

  err = obj.delete_object()
  if err != nil {
    if strings.Contains(err.Error(), "404") {
      /* 404 means it doesn't exist. Call that good enough */
      err = nil
    }
  }
  return err
}

func resourceRestApiExists(d *schema.ResourceData, meta interface{}) (b bool, e error) {
  exists := false
  obj, err := make_api_object(d, meta)
  if err != nil { return false, err }
  log.Printf("resource_api_object.go: Exists routine called. Object built: %s\n", obj.toString())

  err = obj.read_object()
  /* Assume all errors indicate the object just doesn't exist.
     This may not be a good assumption... */
  if err == nil {
    exists = true
  }
  return exists, nil
}
