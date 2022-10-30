'use strict';

const objectsCache = new Map()
const https = {
  request: (options, callback) => new HttpRequest({ options, callback })
};

class HttpRequest {
  _options
  _callback
  _handlers = new Map()
  constructor({ options, callback }) {
    this._options = options;
    this._callback = callback;
  }
  end = () => fetch("https://" + this._options.host + this._options.path)
    .then(res => ({ res, _: this._callback({
      on: (evt, handler) => this._handlers[evt] = handler
    }) }))
    .then(({ res }) => res.text())
    .then(data => this._handlers["data"](data))
    .then(() => this._handlers['end']())
}

class Service {
  _document = {}

  constructor({ document }) {
    this._document = document
  }

  document = () => new Promise(res => res({ serviceDocument: this._document }))

  bounce = (path, options) => this.region()
      .then(({ region }) => region.document())
      .then(({ regionDocument }) => fetch('https://' + regionDocument.origin + path, options))

  region = () => getObject({ path: 'renewable-report/report' })
      .then(report => pickRegion({ report, service: this._document }))
      .then(({ region }) => ({ region: new Region({ document: this._document.regions.find(r => r.region === region) }) }))
}

class Region {
  _document = {}
  constructor({ document }) {
    this._document = document;
  }
  document = () => new Promise(res => res({ regionDocument: this._document }))

  compare = ({ region }) => new Promise(res => res({ difference: 123 }))
}

class Hedge {
  _service
  constructor({ service }) {
    this._service = service;
  }
  bounce = (path, options) => this._service.bounce(path, options)

  service = () => new Promise(res => res({ service: this._service }))

  region = () => this._service.region()

}

export const hedge = ({ service }) => getObject({ path: 'services/' + service }).then(document => new Hedge({ service: new Service({ document }) }))

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

async function getObject({ path, stage }) {
  let domain = 'store.hedge.earth';
  let pathPrefix = '/';
  /*if (stage === 'dev' || stage === undefined) {
      domain = "rollthecloudinc.github.io"
      pathPrefix = "/hedge-objects/"
  }*/
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
  console.log("provideFeedback")
  if (bestRegions.length > 0 && bestRegions[0].region === bestRegion.region) {
      console.log('Using region [' + bestRegion.region + '] with absolute lowest carbon intentity.');
  } else if (bestRegions.length > 0 && bestRegions.find(r => r.region === bestRegion.region) !== undefined) {
      console.log('Using less optimal region [' + bestRegion.region + ']. If you were using [' + bestRegions[0].region + '] could have saved an extra ' + (report.intensities[bestRegion.region] - report.intensities[bestRegions[0].region]) + ' of carbon.');
  } else {
      cobnsole.log('Using region [' + bestRegion.region + '] no intensity grid data available.');
  }
  // @todo: default region difference. - carbon savings.
}