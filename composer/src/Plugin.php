<?php

declare(strict_types=1);

namespace Netgen\Ngcheck\Composer;

use Composer\Composer;
use Composer\EventDispatcher\EventSubscriberInterface;
use Composer\Factory;
use Composer\IO\IOInterface;
use Composer\Plugin\PluginInterface;
use Composer\Script\ScriptEvents;
use Composer\Util\ProcessExecutor;

use function dirname;
use function file_get_contents;
use function is_array;
use function is_file;
use function is_string;
use function json_decode;
use function realpath;
use function rtrim;

/**
 * After "composer install" and "composer update", downloads the ngcheck release matching the
 * installed package version (vendor/bin/ngcheck runs it). When the package is added to a
 * project, also sets the project up with "ngcheck init".
 */
final class Plugin implements PluginInterface, EventSubscriberInterface
{
    private const PACKAGE_NAME = 'netgen/ngcheck';

    private Composer $composer;

    private IOInterface $io;

    private bool $firstInstall = false;

    public function activate(Composer $composer, IOInterface $io): void
    {
        $this->composer = $composer;
        $this->io = $io;

        // Composer writes vendor/composer/installed.json after installing all packages: if the
        // package is not listed there yet, this command installs it into vendor/ for the first time.
        $this->firstInstall = !$this->isListedInInstalledJson();
    }

    public function deactivate(Composer $composer, IOInterface $io): void {}

    public function uninstall(Composer $composer, IOInterface $io): void {}

    public static function getSubscribedEvents(): array
    {
        return [
            ScriptEvents::POST_INSTALL_CMD => 'onInstall',
            ScriptEvents::POST_UPDATE_CMD => 'onUpdate',
        ];
    }

    /**
     * "composer install": the project is already set up (or deliberately not), only install the binaries.
     */
    public function onInstall(): void
    {
        $this->installBinaries();
    }

    /**
     * "composer require" (and "composer update"): when this adds ngcheck to the project, set it up.
     */
    public function onUpdate(): void
    {
        $binary = $this->installBinaries();

        if ($binary === null || !$this->firstInstall) {
            return;
        }

        $this->firstInstall = false;

        if (!$this->io->isInteractive()) {
            $this->io->write('<info>netgen/ngcheck: run "vendor/bin/ngcheck init" to set up the git hook and the pull request check.</info>');

            return;
        }

        $this->runInit($binary);
    }

    private function installBinaries(): ?string
    {
        $package = $this->composer->getRepositoryManager()->getLocalRepository()->findPackage(self::PACKAGE_NAME, '*');

        if ($package === null) {
            return null;
        }

        $installer = new BinaryInstaller(
            $this->composer->getLoop()->getHttpDownloader(),
            $this->io,
            (string) $this->composer->getInstallationManager()->getInstallPath($package),
        );

        return $installer->install($package);
    }

    private function runInit(string $binary): void
    {
        $projectDirectory = dirname((string) realpath(Factory::getComposerFile()));
        $executor = new ProcessExecutor($this->io);
        $output = '';

        $exitCode = $executor->execute(
            ProcessExecutor::escape($binary) . ' init ' . ProcessExecutor::escape($projectDirectory),
            $output,
        );

        $this->io->write(rtrim((string) $output));

        if ($exitCode !== 0) {
            $this->io->writeError(rtrim($executor->getErrorOutput()));
            $this->io->writeError('<warning>netgen/ngcheck: setting up the project failed, fix the problem and run "vendor/bin/ngcheck init".</warning>');
        }
    }

    private function isListedInInstalledJson(): bool
    {
        $installedJson = $this->composer->getConfig()->get('vendor-dir') . '/composer/installed.json';
        $content = is_file($installedJson) ? file_get_contents($installedJson) : false;
        $installed = is_string($content) ? json_decode($content, true) : null;

        if (!is_array($installed)) {
            return false;
        }

        foreach (is_array($installed['packages'] ?? null) ? $installed['packages'] : $installed as $package) {
            if (is_array($package) && ($package['name'] ?? null) === self::PACKAGE_NAME) {
                return true;
            }
        }

        return false;
    }
}
