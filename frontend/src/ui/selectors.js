import {
  getFormValues,
  isValid
} from 'redux-form'
import APISelector from 'template-ui/lib/plugins/api/selectors'
export { default as router } from 'template-ui/lib/plugins/router/selectors'

import forms from './forms'
import config from './config'
import labels from './utils/labels'
import listUtils from './utils/list'

export const valuesSelector = (state) => state.value || {}
export const valueSelector = (state, name) => valuesSelector(state)[name]
export const value = valueSelector
export const routeInfoSelector = (state) => state.router.result

export const formValuesSelector = (name) => {
  const selector = getFormValues(name)
  const handler = (state) => {
    const ret = selector(state)
    return ret || {}
  }
  return handler
}

export const application = {
  servername: (state) => window.location.hostname,
  message: (state) => valueSelector(state, 'applicationMessage')
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
  user: (state) => value(state, config.userValueName),
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
    const allResults = repos.all(state)
    return listUtils.searchObjectList(allResults, search, repo.name)
  },
  pageCurrent: (state) => {
    const st = state.router.params.page || '1'
    const nm = parseInt(st)
    return isNaN(nm) ? 1 : nm
  },
  pageSize: (state) => config.repolist.pageSize,
  count: (state) => repos.all(state).length,
  searchCount: (state) => {
    const results = repos.searchResults(state)
    return results.length
  },
  pageCount: (state) => {
    const results = repos.searchResults(state)
    return Math.ceil(results.length/repos.pageSize())
  },
  // filter the search results (which could be all) through the page grouper
  pageResults: (state) => {
    const searchResults = repos.searchResults(state)
    const pageSize = repos.pageSize(state)
    const pageCurrent = repos.pageCurrent(state)
    const startIndex = (pageCurrent - 1) * pageSize
    return searchResults.slice(startIndex, startIndex + pageSize)
  },
  // grab the Namespace and Name from the url
  extractUrlName: (state) => {
    const routerState = state.router
    const urlChunk = routerState.params._ || ''
    const parts = urlChunk.split('/')
    const Namespace = parts.shift()
    const Name = parts.shift()
    const Section = parts.join('/')
    const Parts = parts
    return {
      Namespace,
      Name,
      Section,
      Parts
    }
  },
  extractBranchName: (state) => {
    const urlName = repos.extractUrlName(state)
    let branch = null
    if(urlName.Parts[0] == 'tree') branch = urlName.Parts[1]
    branch = branch || 'master'
    return branch
  },
  // Name is {Name,Namespace}
  get: (state, Name) => {
    return repos.all(state).filter(data => {
      return repo.name(data) == Name.Name && repo.namespace(data) == Name.Namespace
    })[0]
  },
  getFromUrl: (state) => {
    const urlName = repos.extractUrlName(state)
    return repos.get(state, urlName)
  },
  getBranchFromUrl: (state) => {
    const branchName = repos.extractBranchName(state)
    const repoData = repos.getFromUrl(state)
    return repo.getBranch(repoData, branchName)
  },
  // Name is {Name,Namespace}
  exists: (state, Name) => repos.get(state, Name) ? true : false
}

export const repo = {
  top: (data = {}) => (data || {}).TopLevelVolume || {},
  id: (data = {}) => repo.top(data).Id,
  fullname: (data = {}) => repo.top(data).Name || {},
  names: (data = {}, joinst = '') => {
    const fullname = repo.fullname(data)
    return [fullname.Namespace, fullname.Name].join(joinst)
  },
  title: (data = {}) => repo.names(data, ' / '),
  url: (data = {}) => repo.names(data, '/'),
  name: (data = {}) => repo.fullname(data).Name,
  namespace: (data = {}) => repo.fullname(data).Namespace,
  size: (data = {}) => repo.top(data).SizeBytes,
  serverStatuses: (data = {}) => repo.top(data).ServerStatuses || {},
  sizeTitle: (data = {}) => labels.size(repo.size(data)),
  isPrivate: (data = {}) => true,
  branches: (data = {}) => data.CloneVolumes || [],
  branchList: (data = {}) => {
    const branches = repo.branches(data).map(branchData => {
      return {
        id: branch.id(branchData),
        name: branch.name(branchData)
      }
    })
    return [{
      id: repo.id(data),
      name: 'master'
    }].concat(branches)
  },
  getBranch: (data = {}, name = 'master') => {
    if(name == 'master') return repo.top(data)
    return repo.branches(data).filter(branchData => branch.name(branchData) == name)[0]
  },
  branchCount: (data = {}) => (repo.branches(data).length + 1),
  branchCountTitle: (data = {}) => {
    const count = repo.branchCount(data)
    return `${ count } branch${ count==1 ? '' : 'es' }`
  }
}

export const branch = {
  id: (data) => data.Id,
  name: (data) => data.Clone
}

export const help = {
  currentPage: (state) => {
    const params = state.router.params || {}
    return params._
  },
  variables: (state) => {
    return {
      USER_NAME: auth.name(state),
      SERVER_NAME: application.servername(state)
    }
  }
}