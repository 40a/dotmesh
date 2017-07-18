import React, { Component, PropTypes } from 'react'

import { processRoutes } from 'template-ui/lib/utils/routes'

import RouteFactory from 'template-ui/lib/containers/Route'
import UserWrapper from 'template-ui/lib/containers/UserWrapper'

import Section from 'template-ui/lib/components/Section'

import Application from './containers/Application'
import LoginForm from './containers/LoginForm'
import RegisterForm from './containers/RegisterForm'

import GuestHome from './components/GuestHome'
import UserHome from './components/UserHome'
import Help from './components/Help'
import About from './components/About'

import config from './config'

const Route = RouteFactory(config.basepath)

export const routeConfig = processRoutes({
  '/': {},
  '/help': {
    triggers: []
  },
  '/about': {
  },
  '/login': {
  },
  '/register': {
  }
}, config.basepath)

export const routes = (
  <div>
    <Application>
      <Route home>
        <Section>
          <UserWrapper loggedIn={ false }>
            <GuestHome />
          </UserWrapper>
          <UserWrapper loggedIn={ true }>
            <UserHome />
          </UserWrapper>
        </Section>
      </Route>

      <Route path='/help'>
        <div>
          Help
        </div>
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