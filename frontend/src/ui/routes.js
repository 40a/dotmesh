import React, { Component, PropTypes } from 'react'

import UserWrapper from 'template-ui/lib/plugins/auth/UserWrapper'
import RouteFactory from 'template-ui/lib/plugins/router/Route'
import { processRoutes } from 'template-ui/lib/utils/routes'

import Section from 'template-ui/lib/components/Section'

import config from './config'

import Application from './containers/Application'
import LoginForm from './containers/LoginForm'
import RegisterForm from './containers/RegisterForm'
import RepoForm from './containers/RepoForm'
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
  '/repos/new': {
    user: true,
    authRedirect: '/login',
    hooks: ['repoFormInitialize']
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
  },
  '/*': {
    TEST: 10
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

      <Route path='/dashboard' exact>
        <Section>
          <UserLayout>
            <SectionTabs
              active={0}
            />
          </UserLayout>
        </Section>
      </Route>

      <Route path='/payment' exact>
        <Section>
          <PaymentPage />
        </Section>
      </Route>

      <Route path='/repos' exact>
        <Section>
          <UserLayout>
            <SectionTabs
              active={0}
            />
          </UserLayout>
        </Section>
      </Route>

      <Route path='/repos/new' exact>
        <Section>
          <UserLayout>
            <RepoForm
              title='Create Repository'
            >
            </RepoForm>
          </UserLayout>
        </Section>
      </Route>

      <Route route='/repos/page/:page' exact>
        <Section>
          <UserLayout>
            <SectionTabs
              active={0}
            />
          </UserLayout>
        </Section>
      </Route>

      <Route path='/servers' exact>
        <Section>
          <UserLayout>
            <SectionTabs
              active={1}
            />
          </UserLayout>
        </Section>
      </Route>

      <Route path='/login' exact>
        <Section>
          <LoginForm />
        </Section>
      </Route>

      <Route path='/register' exact>
        <Section>
          <RegisterForm />
        </Section>
      </Route>      

    </Application>
  </div>
)
