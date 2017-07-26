import { createAction } from 'redux-act'

import ApiActions from 'template-ui/lib/plugins/api/actions'
import RouterActions from 'template-ui/lib/plugins/router/actions'
import ValueActions from 'template-ui/lib/plugins/value/actions'

export const value = ValueActions

export const events = {
  menuClick: createAction('menu click')
}

export const router = RouterActions