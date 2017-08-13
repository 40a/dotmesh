import tools from './tools'

export const list = (payload) => {
  return tools.rpc({
    method: 'AllVolumesAndClones',
    headers: getHeaders(payload.credentials),
    httpParams: {
      disableBasicAuth: 'y'
    }
  })
}

const volumeApi = {
  list
}

export default volumeApi