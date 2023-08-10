/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';

export default class OnboardingInstallLinuxIndex extends Route {
  redirect(): void {
    this.transitionTo('onboarding.install.linux.ubuntu');
  }
}
