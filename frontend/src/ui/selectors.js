import {
  getFormValues,
  isValid
} from 'redux-form'
import APISelector from 'template-ui/lib/plugins/api/selectors'
export { default as router } from 'template-ui/lib/plugins/router/selectors'

import forms from './forms'
import config from './config'
import labels from './utils/labels'

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

export const repos = {
  all: (state) => valueSelector(state, 'repos') || [],
  search: (state) => valueSelector(state, 'repoListSearch') || '',
  searchResults: (state) => {
    const search = repos.search(state)
    const useSearch = search.toLowerCase().replace(/\W/g, '')
    const allResults = repos.all(state)
    if(!search) return allResults
    return allResults.filter(data => {
      const useName = (repo.name(data) || '').toLowerCase().replace(/\W/g, '')
      return useName.indexOf(useSearch) >= 0
    })
  },
  currentPage: (state) => {
    const st = state.router.query.page || '1'
    const nm = parseInt(st)
    return isNaN(nm) ? 1 : nm
  },
  pageSize: (state) => config.repolist.pageSize,
  pageCount: (state) => {
    const results = repos.searchResults(state)
    return Math.ceil(results/repos.pageSize())
  },
  // filter the search results (which could be all) through the page grouper
  pageResults: (state) => {
    const searchResults = repos.searchResults(state)
    const pageSize = repos.pageSize(state)
    const currentPage = repos.currentPage(state)
    const startIndex = (currentPage - 1) * pageSize
    return searchResults.slice(startIndex, startIndex + pageSize)
  }
}

export const repo = {
  top: (data) => data.TopLevelVolume,
  id: (data) => repo.top(data).Id,
  name: (data) => repo.top(data).Name,
  size: (data) => repo.top(data).SizeBytes,
  serverStatuses: (data) => repo.top(data).ServerStatuses || {},
  sizeTitle: (data) => labels.size(repo.size(data)),
  isPrivate: (data) => true,
  branches: (data) => data.CloneVolumes || [],
  branchCount: (data) => (repo.branches(data).length + 1),
  branchCountTitle: (data) => {
    const count = repo.branchCount(data)
    return `${ count } branch${ count==1 ? '' : 'es' }`
  }
}