/*
Copyright 2015 Gravitational, Inc.

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

import { Store, toImmutable } from 'nuclear-js';
import moment from 'moment';
import { TLPT_STORED_SESSINS_FILTER_SET_RANGE } from './actionTypes';

export default Store({
  getInitialState() {
    const end = moment(new Date()).endOf('day').toDate();
    const start = moment(end).subtract(3, 'day').startOf('day').toDate();
    const state = {
      start,
      end
    }

    return toImmutable(state);
  },

  initialize() {
    this.on(TLPT_STORED_SESSINS_FILTER_SET_RANGE, setRange);
  }
})

function setRange(state, newState){
  return state.merge(newState);
}
