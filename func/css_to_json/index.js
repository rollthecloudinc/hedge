var cssjson = require("cssjson");

module.exports.handler = function(event, context, callback) {
  console.log('hello world');
  console.log(event);
  console.log(context);
  var json = cssjson.toJSON(event.Content);
  console.log(json);
  callback(null, json);
}