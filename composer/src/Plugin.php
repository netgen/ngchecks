<?php

declare(strict_types=1);

namespace Netgen\Ngcheck\Composer;

use Composer\Composer;
use Composer\EventDispatcher\EventSubscriberInterface;
use Composer\IO\IOInterface;
use Composer\Plugin\PluginInterface;
use Composer\Script\ScriptEvents;

/**
 * Downloads the ngcheck release matching the installed package version after
 * "composer install" and "composer update". vendor/bin/ngcheck runs it.
 */
final class Plugin implements PluginInterface, EventSubscriberInterface
{
    private const PACKAGE_NAME = 'netgen/ngcheck';

    private Composer $composer;

    private IOInterface $io;

    public function activate(Composer $composer, IOInterface $io): void
    {
        $this->composer = $composer;
        $this->io = $io;
    }

    public function deactivate(Composer $composer, IOInterface $io): void {}

    public function uninstall(Composer $composer, IOInterface $io): void {}

    public static function getSubscribedEvents(): array
    {
        return [
            ScriptEvents::POST_INSTALL_CMD => 'installBinaries',
            ScriptEvents::POST_UPDATE_CMD => 'installBinaries',
        ];
    }

    public function installBinaries(): void
    {
        $package = $this->composer->getRepositoryManager()->getLocalRepository()->findPackage(self::PACKAGE_NAME, '*');

        if ($package === null) {
            return;
        }

        $installer = new BinaryInstaller(
            $this->composer->getLoop()->getHttpDownloader(),
            $this->io,
            (string) $this->composer->getInstallationManager()->getInstallPath($package),
        );

        $installer->install($package);
    }
}
