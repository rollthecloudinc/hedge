const CDP = require('chrome-remote-interface')
const launchChrome = require('@serverless-chrome/lambda')

module.exports.handler = async function(event, context, callback) {

  console.log('top handler');

  const queryStringParameters = event.queryStringParameters || {}
  const {
    url = 'https://github.com/adieuadieu/serverless-chrome',
    mobile = false,
  } = queryStringParameters

  let data

  log('Processing screenshot capture for', url)

  const startTime = Date.now()

  try {
    data = await screenshot(url, mobile)
  } catch (error) {
    console.error('Error capturing screenshot for', url, error)
    return callback(error)
  }

  log(`Chromium took ${Date.now() - startTime}ms to load URL and capture screenshot.`)

  return callback(null, {
    statusCode: 200,
    body: data,
    isBase64Encoded: true,
    headers: {
      'Content-Type': 'image/png',
    },
  })
}

async function screenshot(url, mobile = false) {

  console.log('top screenshot');

  const LOAD_TIMEOUT = process.env.PAGE_LOAD_TIMEOUT || 1000 * 60

  let result
  let loaded = false

  const loading = async (startTime = Date.now()) => {
    if (!loaded && Date.now() - startTime < LOAD_TIMEOUT) {
      await sleep(100)
      await loading(startTime)
    }
  }

  console.log('right before cdp');

  const [tab] = await Cdp.List()
  const client = await Cdp({ host: '127.0.0.1', target: tab })

  const {
    Network, Page, Runtime, Emulation,
  } = client

  Network.requestWillBeSent((params) => {
    log('Chrome is sending request for:', params.request.url)
  })

  Page.loadEventFired(() => {
    loaded = true
  })

  try {
    await Promise.all([Network.enable(), Page.enable()])

    if (mobile) {
      await Network.setUserAgentOverride({
        userAgent:
          'Mozilla/5.0 (iPhone; CPU iPhone OS 10_0_1 like Mac OS X) AppleWebKit/602.1.50 (KHTML, like Gecko) Version/10.0 Mobile/14A403 Safari/602.1',
      })
    }

    await Emulation.setDeviceMetricsOverride({
      mobile: !!mobile,
      deviceScaleFactor: 0,
      scale: 1, // mobile ? 2 : 1,
      width: mobile ? 375 : 1280,
      height: 0,
    })

    await Page.navigate({ url })
    await Page.loadEventFired()
    await loading()

    const {
      result: {
        value: { height },
      },
    } = await Runtime.evaluate({
      expression: `(
        () => ({ height: document.body.scrollHeight })
      )();
      `,
      returnByValue: true,
    })

    await Emulation.setDeviceMetricsOverride({
      mobile: !!mobile,
      deviceScaleFactor: 0,
      scale: 1, // mobile ? 2 : 1,
      width: mobile ? 375 : 1280,
      height,
    })

    const screenshot = await Page.captureScreenshot({ format: 'png' })

    result = screenshot.data
  } catch (error) {
    console.error(error)
  }

  await client.close()

  return result
}

function sleep (miliseconds = 100) {
  return new Promise(resolve => setTimeout(() => resolve(), miliseconds))
}


function log (...stuffToLog) {
  if (process.env.LOGGING) {
    console.log(...stuffToLog)
  }
}

/*function launch ({ callback }) {
  launchChrome({
    flags: ['--window-size=1280,1696', '--hide-scrollbars']
  })
  .then((chrome) => {
    // Chrome is now running on localhost:9222

    CDP.Version()
      .then((versionInfo) => {
        callback(null, {
          versionInfo,
        })
      })
      .catch((error) => {
        callback(error)
      })
  })
  // Chrome didn't launch correctly 😢
  .catch(callback)
}*/