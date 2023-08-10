/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

// labelSelect
export function labelSelect([labels, key]: [Array<Array<string>>, string]): string {
  let val = '';

  labels.forEach(function (l) {
    if (l[0] == key) {
      val = l[1];
    }
  });

  return val;
}

export default helper(labelSelect);
