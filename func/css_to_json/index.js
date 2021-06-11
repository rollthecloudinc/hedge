var cssparser = require("cssparser");

module.exports.handler = function(event, context, callback) {
  console.log('hello world');
  console.log(event);
  console.log(context);
  callback();
}