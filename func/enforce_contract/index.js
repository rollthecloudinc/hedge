const Ajv = require("ajv")
const ajv = new Ajv() // options can be passed, e.g. {allErrors: true}

module.exports.handler = function(event, context, callback) {
  console.log('hello world');
  console.log(event);
  console.log(context);

  /*const schema = {
    type: "object",
    properties: {
      foo: {type: "integer"},
      bar: {type: "string"}
    },
    required: ["foo"],
    additionalProperties: false
  }*/

  const contract = event.Contract;
  const schema = contract.schema;
  if (typeof(schema['$schema']) !== 'undefined') {
    delete schema['$schema'];
  }

  console.log('schema', schema);
  
  const validate = ajv.compile(schema)

  /*const data = {
    foo: "blah",
    bar: "abc"
  }*/

  const data = event.Entity;
  console.log('entity', data);
  
  const valid = validate(data)
  console.log('errors', validate.errors)

  const res = {
    Entity: { ...data, userId: event.UserId },
    Valid: validate.errors == null,
    Unauthorized: false,
    Errors: validate.errors
  };

  //var json = cssjson.toJSON(event.Content, { comments: true });
  //console.log(json);

  callback(null, res);
}