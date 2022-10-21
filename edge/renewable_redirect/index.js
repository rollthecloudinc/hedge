'use strict';

exports.handler = (event, context, callback) => {
    const request = event.Records[0].cf.request;
    const headers = request.headers;

    /*if (headers['cloudfront-is-mobile-viewer'] && headers['cloudfront-is-mobile-viewer'][0].value === 'true') {
        request.uri = '/lite + request.uri;
    }*/
    console.log('we are in business!');
    console.log('request', request);

    callback(null, request);
};