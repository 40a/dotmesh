import React, { Component, PropTypes } from 'react'

import UserWrapper from 'template-ui/lib/plugins/auth/UserWrapper'
import RouteFactory from 'template-ui/lib/plugins/router/Route'
import { processRoutes } from 'template-ui/lib/utils/routes'

import Section from 'template-ui/lib/components/Section'

import config from './config'

import Application from './containers/Application'
import LoginForm from './containers/LoginForm'
import RegisterForm from './containers/RegisterForm'
import VolumeTable from './containers/VolumeTable'
import ServerTable from './containers/ServerTable'
import PaymentPage from './containers/PaymentPage'

import Home from './components/Home'
import Dashboard from './containers/Dashboard'

const Route = RouteFactory(config.basepath)

export const routeConfig = processRoutes({
  '/': {
    controlLoopHooks: [
      'volumeList'
    ]
  },
  '': {
    controlLoopHooks: [
      'volumeList'
    ]
  },
  '/help': {},
  '/dashboard': {
    user: true,
    authRedirect: '/login',
    controlLoopHooks: [
      'volumeList'
    ]
  },
  '/servers': {
    user: true,
    authRedirect: '/login',
    controlLoopHooks: [
      'volumeList'
    ]
  },
  '/volumes': {
    user: true,
    authRedirect: '/login',
    controlLoopHooks: [
      'volumeList'
    ]
  },
  '/payment': {
    user: true,
    authRedirect: '/login'
  },
  '/login': {
    user: false,
    authRedirect: '/dashboard'
  },
  '/register': {
    user: false,
    authRedirect: '/dashboard'
  }
}, config.basepath)

export const routes = (
  <div>
    <Application>
      <Route home>
        <Section>
          <UserWrapper loggedIn={ false }>
            <Home />
          </UserWrapper>
          <UserWrapper loggedIn={ true }>
            <Section>
              <Dashboard />
            </Section>
          </UserWrapper>
        </Section>
      </Route>

      <Route path='/help'>
        <div>
          Help
        </div>
      </Route>

      <Route path='/dashboard'>
        <Section>
          <Dashboard />
        </Section>
      </Route>

      <Route path='/payment'>
        <Section>
          <PaymentPage />
        </Section>
      </Route>

      <Route path='/volumes'>
        <Section>
          <Dashboard />
        </Section>
      </Route>

      <Route path='/servers'>
        <Section>
          <ServerTable />
        </Section>
      </Route>

      <Route path='/login'>
        <Section>
          <LoginForm />
        </Section>
      </Route>

      <Route path='/register'>
        <Section>
          <RegisterForm />
        </Section>
      </Route>      

    </Application>
  </div>
)
