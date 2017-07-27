import processLoaders from 'template-ui/lib/plugins/api/processLoaders'
import auth from './auth'
import volume from './volume'

// a combo of handler, actions and saga
const loaders = {
  authStatus: auth.status,
  authRegister: auth.register,
  volumeList: volume.list
}

const processedLoaders = processLoaders(loaders)
const apis = processedLoaders.apis

export const actions = processedLoaders.actions
export default apis