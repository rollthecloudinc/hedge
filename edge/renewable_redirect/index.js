'use strict';

const https = require('https');

const objectsCache = new Map();

exports.handler = async (event, _, callback) => {
    console.log('region', process.env.AWS_REGION);
    const request = event.Records[0].cf.request;
    const pieces = request.uri.split('/')
    console.log('pieces', pieces.join('/'));
    const uri = "/" + pieces.slice(2).join('/')
    const report = await getObject({ path: 'renewable-report/report' });
    const service = await getObject({ path: 'services/' + pieces[1] });
    const bestRegion = pickRegion({ service, report });
    const bestRegions = calculateBestRegions({ report });
    provideFeedback({ bestRegions, bestRegion, report });
    console.log('herex');
    delete request.origin.s3
    const customHeaders = {};
    customHeaders["x-hedge-regions"] = [{ key: "x-hedge-regions", value: Object.keys(report.intensities).join(",") }];
    customHeaders["x-hedge-intensities"] = [{ key: "x-hedge-intensities", value: Object.keys(report.intensities).map(r => report.intensities[r]).join(",") }];
    customHeaders["x-hedge-region"] = [{ key: "x-hedge-region", value: bestRegion.region }];
    customHeaders["x-hedge-service"] = [{ key: "x-hedge-service", value: pieces[1] }];
    request.origin.custom = {
      domainName: bestRegion.origin,
      port: 443,
      protocol: "https",
      path: "",
      sslProtocols: ["TLSv1", "TLSv1.1", "TLSv1.2"],
      readTimeout: 30,
      keepaliveTimeout: 30,
      customHeaders
    };
    request.uri = uri
    request.headers["host"] = [{key: "host", value: bestRegion.origin }];
    request.headers['cache-control'] = [{key: "cache-control", value: 'no-cache'}];
    console.log('new request', request);
    callback(null, request);
};

function pickRegion({ service, report }) {
    const availableRegions = service.regions.map(r => r.region);
    const bestRegions = calculateBestRegions({ report });
    let bestAvailableRegion = pickBestAvailableRegion({ availableRegions, bestRegions });
    if (bestAvailableRegion === undefined) {
        bestAvailableRegion = report.defaultRegion !== undefined ? report.defaultRegion : 'eastus';
    }
    const bestServiceRegion = service.regions.find(r => r.region === bestAvailableRegion);
    console.log('pickRegion', 'best service region', bestServiceRegion.region);
    console.log('pickRegion', 'picked region intensity', report.intensities[bestAvailableRegion]);
    return { origin: bestServiceRegion.origin, region: bestServiceRegion.region };
}

async function getObject({ path }) {
    const stage = process.env.STAGE;
    let domain = 'store.hedge.earth';
    let pathPrefix = '/';
    //console.log('vars', process.env);
    console.log('stage', stage);
    //if (stage === 'dev' || stage === undefined) {
        domain = "rollthecloudinc.github.io";
        pathPrefix = "/hedge-objects/";
    //}
    const options = { host: domain, path: pathPrefix + path + '.json' };
    console.log('getObject', "options", options);
    return objectsCache.has(path) ? new Promise(res => res(objectsCache.get(path))) : new Promise(resolve => {
        https.request(options, res => {
            var str = '';
            res.on('data', chunk => str += chunk);
            res.on('end', () => {
                console.log('getObject', "path", str);
                const obj = JSON.parse(str);
                objectsCache.set(path, obj);
                resolve(obj);
            });
        }).end();
    });
}

function calculateBestRegions({ report }) {
    const intensitiesKeys = Object.keys(report.intensities);
    const bestRegions = [];
    intensitiesKeys.forEach(region => {
        if (report.intensities[region] > 0) {
            bestRegions.push({ intensity: report.intensities[region], region });
        } else {
            console.log('pickRegion', 'toss', region);
        }
    });
    bestRegions.sort((a, b) => a.intensity > b.intensity ? 1 : -1);
    console.log('calculateBestRegions', 'best regions', bestRegions);
    return bestRegions;
}

function pickBestAvailableRegion({ bestRegions, availableRegions }) {
    let bestAvailableRegion
    const len = bestRegions.length;
    for (let i = 0; i < len; i++) {
        const matchedAvailabilityRegion = availableRegions.find(region => region === bestRegions[i].region);
        if (matchedAvailabilityRegion !== undefined) {
            bestAvailableRegion = matchedAvailabilityRegion;
            break;
        }
    }
    console.log('pickBestAvailableRegion', 'best region', bestAvailableRegion);
    return bestAvailableRegion;
}

function provideFeedback({ bestRegions, bestRegion, report }) {
    console.log("provideFeedbackx")
    if (bestRegions.length > 0 && bestRegions[0].region === bestRegion.region) {
        console.log('Using region [' + bestRegion.region + '] with absolute lowest carbon intentity.');
    } else if (bestRegions.length > 0 && bestRegions.find(r => r.region === bestRegion.region) !== undefined) {
        console.log('Using less optimal region [' + bestRegion.region + ']. If you were using [' + bestRegions[0].region + '] could have saved an extra ' + (report.intensities[bestRegion.region] - report.intensities[bestRegions[0].region]) + ' of carbon.');
    } else {
        cobnsole.log('Using region [' + bestRegion.region + '] no intensity grid data available.');
    }
    // @todo: default region difference. - carbon savings.
}