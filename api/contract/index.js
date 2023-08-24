	
// handler.js
'use strict';
 
module.exports.handler = function (context, req) {
  const query = req.query; // dictionary of query strings
  const body = req.body; // Parsed body based on content-type
  const method = req.method; // HTTP Method (GET, POST, PUT, etc.)
  const originalUrl = req.originalUrl; // Original URL of the request - https://myapp.azurewebsites.net/api/foo?code=sc8Rj2a7J
  const headers = req.headers; // dictionary of headers
  const params = req.params; // dictionary of params from URL
  const rawBody = req.rawBody; // unparsed body
 
  context.res = {
    headers: {
      'content-type': 'application/json',
    },
    body: {
      hello: 'world',
    },
  };
  context.done();
};