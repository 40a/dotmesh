// base64 encode the given username / password combo - this is what we send in the header
const encodeCredentials = (username, password) => new Buffer(username + ':' + password).toString('base64')

// given an encoded username/password combo - produce the HTTP headers we should send with a request
const getHeaders = (credentials) => {
  return {
    Authorization: `Basic ${credentials}`
  }
}

const tools = {
  encodeCredentials,
  getHeaders
}

export default tools