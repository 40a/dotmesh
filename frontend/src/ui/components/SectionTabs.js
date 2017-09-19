import React, { Component, PropTypes } from 'react'
import { Tab, Tabs } from 'react-toolbox'

import RepoList from '../containers/RepoList'
import ServerTable from '../containers/ServerTable'

class SectionTabs extends Component {
  render() {
    const active = this.props.active
    return (
      <section>
        <Tabs index={active}>
          <Tab label='Repositories' onClick={ () => this.props.link('/repos') }>
            <RepoList />
          </Tab>
          <Tab label='Servers' onClick={ () => this.props.link('/servers') }>
            <ServerTable />
          </Tab>
        </Tabs>
      </section>
    )
  }
}

export default SectionTabs