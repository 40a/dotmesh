import { createAction } from 'redux-act'

import ApiActions from 'template-ui/lib/plugins/api/actions'
import FormActions from 'template-ui/lib/plugins/form/actions'
import RouterActions from 'template-ui/lib/plugins/router/actions'
import ValueActions from 'template-ui/lib/plugins/value/actions'

import formConfig from './forms'
import config from './config'

export const forms = FormActions(formConfig)

export const value = ValueActions

export const events = {
  menuClick: createAction('menu click')
}

export const router = RouterActions

export const auth = {
  setUser: (credentials) => value.set(config.userValueName, credentials)
}

export const repos = {
  updateSearch: search => value.set('repoListSearch', search)
}

export const commits = {
  updateSearch: search => value.set('repoCommitListSearch', search)
}

export const application = {
  setMessage: (message) => value.set('applicationMessage', message),
  clearMessage: () => value.set('applicationMessage', null)
}