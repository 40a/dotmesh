import React, { Component, PropTypes } from 'react'

import UserWrapper from 'template-ui/lib/plugins/auth/UserWrapper'
import RouteFactory from 'template-ui/lib/plugins/router/Route'
import { processRoutes } from 'template-ui/lib/utils/routes'

import Section from 'template-ui/lib/components/Section'

import config from './config'

import Application from './containers/Application'
import LoginForm from './containers/LoginForm'
import RegisterForm from './containers/RegisterForm'
import PaymentPage from './containers/PaymentPage'
import SectionTabs from './containers/SectionTabs'
import Help from './containers/Help'

import Home from './components/Home'
import UserLayout from './containers/UserLayout'

const Route = RouteFactory(config.basepath)

export const routeConfig = processRoutes({
  '/': {
    controlLoopSaga: 'repoList'
  },
  '': {
    controlLoopSaga: 'repoList'
  },
  '/help': {
    autoScroll: false
  },
  '/help/*': {
    autoScroll: false
  },
  '/dashboard': {
    user: true,
    authRedirect: '/login',
    controlLoopSaga: 'repoList'
  },
  '/servers': {
    user: true,
    authRedirect: '/login',
    controlLoopSaga: 'repoList'
  },
  '/repos': {
    user: true,
    authRedirect: '/login',
    controlLoopSaga: 'repoList'
  },
  '/repos/page/:page': {
    user: true,
    authRedirect: '/login',
    controlLoopSaga: 'repoList'
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
              <UserLayout>
                <SectionTabs
                  active={0}
                />
              </UserLayout>
            </Section>
          </UserWrapper>
        </Section>
      </Route>

      <Route path='/help'>
        <Help />
      </Route>

      <Route path='/dashboard'>
        <Section>
          <UserLayout>
            <SectionTabs
              active={0}
            />
          </UserLayout>
        </Section>
      </Route>

      <Route path='/payment'>
        <Section>
          <PaymentPage />
        </Section>
      </Route>

      <Route path='/repos'>
        <Section>
          <UserLayout>
            <SectionTabs
              active={0}
            />
          </UserLayout>
        </Section>
      </Route>

      <Route path='/repos/page/:page'>
        <Section>
          <UserLayout>
            <SectionTabs
              active={0}
            />
          </UserLayout>
        </Section>
      </Route>

      <Route path='/servers'>
        <Section>
          <UserLayout>
            <SectionTabs
              active={1}
            />
          </UserLayout>
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
