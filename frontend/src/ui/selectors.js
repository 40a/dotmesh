import {
  getFormValues,
  isValid
} from 'redux-form'
import APISelector from 'template-ui/lib/plugins/api/selectors'
export { default as router } from 'template-ui/lib/plugins/router/selectors'

import forms from './forms'
import config from './config'

export const valuesSelector = (state) => state.value || {}
export const valueSelector = (state, name) => valuesSelector(state)[name]
export const value = (name) => (state) => valueSelector(state, name)
export const routeInfoSelector = (state) => state.router.result

export const formValuesSelector = (name) => {
  const selector = getFormValues(name)
  const handler = (state) => {
    const ret = selector(state)
    return ret || {}
  }
  return handler
}

export const billing = {
  plans: (state) => {
    const config = valueSelector(state, 'config') || {}
    return config.Plans || []
  },
  planById: (state, id) => billing.plans(state).filter(plan => plan.Id == id)[0],
  stripeKey: (state) => {
    const config = valueSelector(state, 'config') || {}
    return config.StripePublicKey
  }
}

export const auth = {
  user: value(config.userValueName),
  email: (state) => {
    const user = auth.user(state) || {}
    return user.Email
  },
  emailHash: (state) => {
    const user = auth.user(state) || {}
    return user.EmailHash
  },
  name: (state) => {
    const user = auth.user(state) || {}
    return user.Name
  }
}

export const formValidSelector = (name) => isValid(name)
export const userSelector = (state) => state.values.user

export const api = APISelector()

export const form = Object.keys(forms).reduce((all, name) => {
  all[name] = {
    valid: formValidSelector(name),
    values: formValuesSelector(name)
  }
  return all
}, {})

const mapFilesystem = (data) => {
  return {
    Id: data.Id,
    Name: data.Name,
    Clone: data.Clone,
    Master: data.Master,
    SizeBytes: data.SizeBytes,
    DirtyBytes: data.DirtyBytes,
    CommitCount: data.CommitCount,
    ServerStatuses: data.ServerStatuses
  }   
}

// sub-selector - it operates on a single volume
export const repo = (data) => {
  const CloneVolumes = (data.CloneVolumes || []).map(mapFilesystem)

  // there is always a master branch so +1
  const CloneVolumeCount = (CloneVolumes || []).length + 1
  return {
    TopLevelVolume: mapFilesystem(data.TopLevelVolume),
    CloneVolumes,
    CloneVolumeCount,
    Owner: data.Owner,
    Collaborators: data.Collaborators
  }
}
