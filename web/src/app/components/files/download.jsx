/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { PropTypes } from 'react';
import { Text } from './items';

export class FileDownloadSelector extends React.Component {

  static propTypes = {
    onDownload: PropTypes.func.isRequired,
  }

  state = {
    path: '~/'
  }

  onChangePath = e => {
    this.setState({
      path: e.target.value
    })
  }

  isValidPath(path) {
    return path && path[path.length - 1] !== '/';
  }

  onDownload = () => {
    if (this.isValidPath(this.state.path)) {
      this.props.onDownload(this.state.path)
    }
  }

  onKeyDown = event => {
    if (event.key === 'Enter') {
      event.preventDefault();
      event.stopPropagation();
      this.onDownload();
    }
  }

  moveCaretAtEnd(e) {
    const tmp = e.target.value;
    e.target.value = '';
    e.target.value = tmp;
  }

  render() {
    const { path } = this.state;
    const isBtnDisabled = !this.isValidPath(path);
    return (
      <div className="grv-file-transfer-header m-b">
        <Text className="m-b">
          <h4>SCP DOWNLOAD</h4>
        </Text>
        <Text className="m-b-xs">
          File path
        </Text>
        <div className="grv-file-transfer-download">
          <input onChange={this.onChangePath}
            ref={e => this.inputRef = e}
            value={path}
            className="grv-file-transfer-input m-r-sm"
            autoFocus
            onFocus={this.moveCaretAtEnd}
            onKeyDown={this.onKeyDown}
          />
          <button className="btn btn-sm grv-file-transfer-btn"
            style={{width:"105px"}}
            disabled={isBtnDisabled}
            onClick={this.onDownload}>
            Download
          </button>
        </div>
      </div>
    )
  }
}
