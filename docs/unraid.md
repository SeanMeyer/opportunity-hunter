# Unraid Setup

## Installation

1. Download the template to your Unraid boot drive:

   ```bash
   curl -o /boot/config/plugins/dockerMan/templates-user/my-opportunity-hunter.xml \
     https://raw.githubusercontent.com/seanmeyer/opportunity-hunter/main/unraid/opportunity-hunter.xml
   ```

2. In the Unraid Docker tab, click **Add Container** and select **opportunity-hunter**.

3. Fill in your API keys, home location, and webhook URLs in the form, then click **Apply**.

The image is pulled automatically from `ghcr.io/seanmeyer/opportunity-hunter:latest`.

## Updating

Click the container icon in the Docker tab and select **Update**. Unraid pulls the latest image automatically.
