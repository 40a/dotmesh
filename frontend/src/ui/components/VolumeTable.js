import React, { Component, PropTypes } from 'react'
import { Table, TableHead, TableRow, TableCell } from 'react-toolbox/lib/table'

import labelUtils from '../utils/labels'

class VolumeTable extends Component {
  render() {
    const data = this.props.data || []
    return (
      <div id="volumePage">
        <h1 id="title">Volumes</h1>
        <Table selectable={ false } style={{ marginTop: 10 }}>
          <TableHead>
            <TableCell>Name</TableCell>
            <TableCell>Size</TableCell>
            <TableCell>Branches</TableCell>
          </TableHead>
          {data.map((item, idx) => {
            const volume = item.TopLevelVolume || {}
            const clones = item.CloneVolumes || []
            const CloneVolumeCount = item.CloneVolumeCount
            const branchCountTitle = 'branch' + (CloneVolumeCount == 1 ? '' : 'es')

            return (
              <TableRow key={idx}>
                <TableCell>{volume.Name || ''}</TableCell>
                <TableCell>{labelUtils.size(volume.SizeBytes || 0)}</TableCell>
                <TableCell>{CloneVolumeCount} {branchCountTitle}</TableCell>
              </TableRow>
            )
          })}
        </Table>
      </div>
    )
  }
}

export default VolumeTable