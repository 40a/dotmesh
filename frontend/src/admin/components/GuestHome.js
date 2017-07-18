import React, { Component, PropTypes } from 'react'
import { Grid, Row, Col } from 'react-flexbox-grid'

import colorsTheme from './theme/colors.css'

class Home extends Component {
  render() {
    return (
      <Grid fluid>
        <Row>
          <Col lg={12}>
            <div className={ colorsTheme.redText }>
              This is the homepage with text styled by css - please login
            </div>
          </Col>
        </Row>
      </Grid>
    )
  }
}

export default Home