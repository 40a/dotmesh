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
              <p>This is the admin panel homepage (probably we should just display login)</p>
              <p>The content is styled by CSS - take a look in `components/GuestHome4.js`</p>
            </div>
          </Col>
        </Row>
      </Grid>
    )
  }
}

export default Home